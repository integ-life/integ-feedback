export type Guest = { name?: string; email?: string };
export type Author = { name: string; user_id?: string; registered: boolean };
export type Comment = { id: string; resource: string; parent_id?: string; body: string; author: Author; created_at: string; updated_at: string };
export type FeedbackKind = "idea" | "issue" | "question" | "other";
export type FeedbackReceipt = { id: string; resource: string; kind: FeedbackKind; status: string; has_attachment: boolean; attachment_count: number; created_at: string };
export type PreparedFeedbackImage = { base64: string; width: number; height: number; bytes: number };
export type FeedbackClientOptions = { apiUrl: string; projectKey: string; getAccessToken?: () => string | undefined | Promise<string | undefined>; signal?: AbortSignal };

export class FeedbackClient {
  constructor(private readonly options: FeedbackClientOptions) {}
  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const token = await this.options.getAccessToken?.();
    const controller=new AbortController(), external=init.signal??this.options.signal;
    const abort=()=>controller.abort();
    external?.addEventListener('abort',abort,{once:true});
    if(external?.aborted) controller.abort();
    const timer=setTimeout(abort,30000);
    try {
      const response = await fetch(`${this.options.apiUrl.replace(/\/$/, "")}${path}`, { ...init, signal:controller.signal, headers: { "Content-Type": "application/json", "X-Project-Key": this.options.projectKey, ...(token ? { Authorization: `Bearer ${token}` } : {}), ...init.headers } });
      if (!response.ok) { const detail = await response.json().catch(() => ({})); throw new FeedbackError(response.status, detail?.error?.code ?? "request_failed", detail?.error?.message ?? response.statusText); }
      return response.status === 204 ? undefined as T : await response.json();
    } finally { clearTimeout(timer);external?.removeEventListener('abort',abort); }
  }
  listComments(resource: string, options: { limit?: number; after?: string } = {}) { const q = new URLSearchParams({ resource }); if(options.limit)q.set("limit",String(options.limit));if(options.after)q.set("after",options.after);return this.request<{items:Comment[];next_cursor:string}>(`/v1/comments?${q}`); }
  createComment(input: { resource: string; body: string; parentId?: string; guest?: Guest }) { return this.request<Comment>("/v1/comments", { method:"POST", body:JSON.stringify({resource:input.resource,body:input.body,parent_id:input.parentId??"",guest_name:input.guest?.name??"",guest_email:input.guest?.email??""}) }); }
  deleteComment(id: string) { return this.request<void>(`/v1/comments/${encodeURIComponent(id)}`, {method:"DELETE"}); }
  submitFeedback(input: {resource:string;kind:FeedbackKind;body:string;guest?:Guest;image?:PreparedFeedbackImage;images?:PreparedFeedbackImage[];attachmentConsent?:boolean}) {
    if(input.image && input.images?.length) return Promise.reject(new FeedbackError(400,"invalid_attachment","Use image or images, not both"));
    const images=input.images??(input.image?[input.image]:[]);
    if(images.length>4) return Promise.reject(new FeedbackError(400,"invalid_attachment","At most four images per report are allowed"));
    if(images.length && !input.attachmentConsent) return Promise.reject(new FeedbackError(400,"attachment_consent_required","Explicit attachment consent is required"));
    if(images.some(image=>image.bytes>524288 || image.width>1280 || image.height>1280)) return Promise.reject(new FeedbackError(400,"invalid_attachment","Image exceeds upload limits"));
    const attachments=images.length>1?{images_base64:images.map(image=>image.base64),attachment_consent:true}:images.length?{image_base64:images[0].base64,attachment_consent:true}:{};
    return this.request<FeedbackReceipt>("/v1/feedback",{method:"POST",body:JSON.stringify({resource:input.resource,kind:input.kind,body:input.body,guest_name:input.guest?.name??"",guest_email:input.guest?.email??"",...attachments})});
  }
}
export class FeedbackError extends Error { constructor(public status:number,public code:string,message:string){super(message);this.name="FeedbackError"} }

/** Local preparation only. Upload requires a separate submitFeedback call. */
export async function prepareFeedbackImage(file: Blob): Promise<PreparedFeedbackImage> {
  if(!['image/jpeg','image/png','image/webp'].includes(file.type) || !file.size || file.size>12*1024*1024) throw new Error('Choose a JPEG, PNG or WebP image up to 12 MiB.');
  const bitmap=await createImageBitmap(file);
  try {
    if(bitmap.width*bitmap.height>50_000_000) throw new Error('Image dimensions are too large.');
    const canvas=document.createElement('canvas');
    const context=canvas.getContext('2d');
    if(!context) throw new Error('Image preparation is unavailable.');
    let blob: Blob | null=null;
    for(const edge of [1280,960,640]) {
      const scale=Math.min(1,edge/Math.max(bitmap.width,bitmap.height));
      canvas.width=Math.max(1,Math.round(bitmap.width*scale)); canvas.height=Math.max(1,Math.round(bitmap.height*scale));
      context.fillStyle='#ffffff'; context.fillRect(0,0,canvas.width,canvas.height);
      context.drawImage(bitmap,0,0,canvas.width,canvas.height);
      for(const quality of [.85,.65,.45]) {
        blob=await new Promise<Blob|null>(resolve=>canvas.toBlob(resolve,'image/jpeg',quality));
        if(blob && blob.size<=524288) break;
      }
      if(blob && blob.size<=524288) break;
    }
    if(!blob || blob.size>524288) throw new Error('Could not prepare an image within the upload limit.');
    const encoded=await new Promise<string>((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(String(reader.result).split(',')[1]);reader.onerror=()=>reject(new Error('Could not read prepared image.'));reader.readAsDataURL(blob!);});
    return {base64:encoded,width:canvas.width,height:canvas.height,bytes:blob.size};
  } finally { bitmap.close(); }
}

export function mountFeedback(options: FeedbackClientOptions & {element:HTMLElement;resource:string;mode?:"comments"|"feedback"}) {
  const client=new FeedbackClient(options), root=options.element, mode=options.mode??"comments";
  root.innerHTML=`<form data-form><textarea name="body" required maxlength="10000" placeholder="${mode==="comments"?"Write a comment…":"Share feedback…"}"></textarea><input name="name" maxlength="100" placeholder="Name (optional)"><input name="email" type="email" placeholder="Email (optional, private)">${mode==="feedback"?'<select name="kind"><option value="idea">Idea</option><option value="issue">Issue</option><option value="question">Question</option><option value="other">Other</option></select>':""}<button>Submit</button><output data-status></output></form><div data-list></div>`;
  const form=root.querySelector<HTMLFormElement>("[data-form]")!,status=root.querySelector<HTMLOutputElement>("[data-status]")!,list=root.querySelector<HTMLElement>("[data-list]")!;
  const refresh=async()=>{if(mode!=="comments")return;const data=await client.listComments(options.resource);list.replaceChildren(...data.items.map(c=>{const article=document.createElement("article");const strong=document.createElement("strong");strong.textContent=c.author.name;const p=document.createElement("p");p.textContent=c.body;article.append(strong,p);return article}))};
  form.addEventListener("submit",async e=>{e.preventDefault();status.textContent="Sending…";const d=new FormData(form),guest={name:String(d.get("name")??""),email:String(d.get("email")??"")};try{if(mode==="comments")await client.createComment({resource:options.resource,body:String(d.get("body")),guest});else await client.submitFeedback({resource:options.resource,kind:String(d.get("kind")) as FeedbackKind,body:String(d.get("body")),guest});form.reset();status.textContent="Thanks!";await refresh()}catch(error){status.textContent=error instanceof Error?error.message:"Could not submit"}});void refresh();return {client,refresh,destroy:()=>root.replaceChildren()};
}
