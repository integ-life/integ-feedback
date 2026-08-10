export type Guest = { name?: string; email?: string };
export type Author = { name: string; user_id?: string; registered: boolean };
export type Comment = { id: string; resource: string; parent_id?: string; body: string; author: Author; created_at: string; updated_at: string };
export type FeedbackKind = "idea" | "issue" | "question" | "other";
export type FeedbackClientOptions = { apiUrl: string; projectKey: string; getAccessToken?: () => string | undefined | Promise<string | undefined> };

export class FeedbackClient {
  constructor(private readonly options: FeedbackClientOptions) {}
  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const token = await this.options.getAccessToken?.();
    const response = await fetch(`${this.options.apiUrl.replace(/\/$/, "")}${path}`, { ...init, headers: { "Content-Type": "application/json", "X-Project-Key": this.options.projectKey, ...(token ? { Authorization: `Bearer ${token}` } : {}), ...init.headers } });
    if (!response.ok) { const detail = await response.json().catch(() => ({})); throw new FeedbackError(response.status, detail?.error?.code ?? "request_failed", detail?.error?.message ?? response.statusText); }
    return response.status === 204 ? undefined as T : response.json();
  }
  listComments(resource: string, options: { limit?: number; after?: string } = {}) { const q = new URLSearchParams({ resource }); if(options.limit)q.set("limit",String(options.limit));if(options.after)q.set("after",options.after);return this.request<{items:Comment[];next_cursor:string}>(`/v1/comments?${q}`); }
  createComment(input: { resource: string; body: string; parentId?: string; guest?: Guest }) { return this.request<Comment>("/v1/comments", { method:"POST", body:JSON.stringify({resource:input.resource,body:input.body,parent_id:input.parentId??"",guest_name:input.guest?.name??"",guest_email:input.guest?.email??""}) }); }
  deleteComment(id: string) { return this.request<void>(`/v1/comments/${encodeURIComponent(id)}`, {method:"DELETE"}); }
  submitFeedback(input: {resource:string;kind:FeedbackKind;body:string;guest?:Guest}) { return this.request("/v1/feedback",{method:"POST",body:JSON.stringify({resource:input.resource,kind:input.kind,body:input.body,guest_name:input.guest?.name??"",guest_email:input.guest?.email??""})}); }
}
export class FeedbackError extends Error { constructor(public status:number,public code:string,message:string){super(message);this.name="FeedbackError"} }

export function mountFeedback(options: FeedbackClientOptions & {element:HTMLElement;resource:string;mode?:"comments"|"feedback"}) {
  const client=new FeedbackClient(options), root=options.element, mode=options.mode??"comments";
  root.innerHTML=`<form data-form><textarea name="body" required maxlength="10000" placeholder="${mode==="comments"?"Write a comment…":"Share feedback…"}"></textarea><input name="name" maxlength="100" placeholder="Name (optional)"><input name="email" type="email" placeholder="Email (optional, private)">${mode==="feedback"?'<select name="kind"><option value="idea">Idea</option><option value="issue">Issue</option><option value="question">Question</option><option value="other">Other</option></select>':""}<button>Submit</button><output data-status></output></form><div data-list></div>`;
  const form=root.querySelector<HTMLFormElement>("[data-form]")!,status=root.querySelector<HTMLOutputElement>("[data-status]")!,list=root.querySelector<HTMLElement>("[data-list]")!;
  const refresh=async()=>{if(mode!=="comments")return;const data=await client.listComments(options.resource);list.replaceChildren(...data.items.map(c=>{const article=document.createElement("article");const strong=document.createElement("strong");strong.textContent=c.author.name;const p=document.createElement("p");p.textContent=c.body;article.append(strong,p);return article}))};
  form.addEventListener("submit",async e=>{e.preventDefault();status.textContent="Sending…";const d=new FormData(form),guest={name:String(d.get("name")??""),email:String(d.get("email")??"")};try{if(mode==="comments")await client.createComment({resource:options.resource,body:String(d.get("body")),guest});else await client.submitFeedback({resource:options.resource,kind:String(d.get("kind")) as FeedbackKind,body:String(d.get("body")),guest});form.reset();status.textContent="Thanks!";await refresh()}catch(error){status.textContent=error instanceof Error?error.message:"Could not submit"}});void refresh();return {client,refresh,destroy:()=>root.replaceChildren()};
}
