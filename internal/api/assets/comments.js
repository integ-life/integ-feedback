const API = new URL("/", import.meta.url).origin;

class IntegComments extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    this.attachShadow({ mode: "open" });
    this.render();
    this.load();
  }

  get resource() { return this.getAttribute("resource") || location.pathname; }
  get projectKey() { return this.getAttribute("project-key") || ""; }
  get labels() {
    const zh = (this.getAttribute("lang") || document.documentElement.lang).toLowerCase().startsWith("zh");
    return zh ? { title:"评论", empty:"还没有评论，来说点什么吧。", body:"写下评论…", name:"用户名（可选）", email:"邮箱（可选，不会公开）", send:"发布评论", sending:"发布中…", error:"暂时无法加载评论。", thanks:"评论已发布。", guest:"游客" }
      : { title:"Comments", empty:"No comments yet. Start the conversation.", body:"Write a comment…", name:"Name (optional)", email:"Email (optional, private)", send:"Post comment", sending:"Posting…", error:"Comments are temporarily unavailable.", thanks:"Comment posted.", guest:"Guest" };
  }

  render() {
    const l=this.labels;
    this.shadowRoot.innerHTML=`<style>:host{display:block;margin:3rem 0;color:inherit;font:inherit}.box{border-top:1px solid color-mix(in srgb,currentColor 18%,transparent);padding-top:1.5rem}h2{font:700 1.35rem/1.2 inherit;margin:0 0 1rem}.list{display:grid;gap:.75rem;margin-bottom:1rem}.comment{padding:1rem;border:1px solid color-mix(in srgb,currentColor 15%,transparent);border-radius:12px;background:color-mix(in srgb,currentColor 3%,transparent)}.meta{display:flex;gap:.6rem;align-items:center;font-size:.82rem;opacity:.7}.badge{font-size:.68rem;padding:.12rem .4rem;border:1px solid currentColor;border-radius:999px}.comment p{white-space:pre-wrap;overflow-wrap:anywhere;margin:.55rem 0 0}form{display:grid;gap:.65rem}textarea,input{box-sizing:border-box;width:100%;font:inherit;color:inherit;background:transparent;border:1px solid color-mix(in srgb,currentColor 22%,transparent);border-radius:10px;padding:.75rem}textarea{min-height:7rem;resize:vertical}.identity{display:grid;grid-template-columns:1fr 1fr;gap:.65rem}button{justify-self:start;font:600 .9rem inherit;padding:.65rem 1rem;border:0;border-radius:999px;cursor:pointer;background:#6d5dfc;color:white}button:disabled{opacity:.55;cursor:wait}output,.empty{font-size:.88rem;opacity:.72}@media(max-width:520px){.identity{grid-template-columns:1fr}}</style><section class="box"><h2>${l.title}</h2><div class="list" aria-live="polite"></div><form><textarea name="body" maxlength="10000" required placeholder="${l.body}"></textarea><div class="identity"><input name="name" maxlength="100" autocomplete="nickname" placeholder="${l.name}"><input name="email" maxlength="320" type="email" autocomplete="email" placeholder="${l.email}"></div><button>${l.send}</button><output aria-live="polite"></output></form></section>`;
    this.shadowRoot.querySelector("form").addEventListener("submit", e=>this.submit(e));
  }

  headers() { return { "Content-Type":"application/json", "X-Project-Key":this.projectKey }; }
  async load() {
    const list=this.shadowRoot.querySelector(".list"),l=this.labels;
    try {
      const r=await fetch(`${API}/v1/comments?resource=${encodeURIComponent(this.resource)}&limit=50`,{headers:this.headers()});
      if(!r.ok) throw new Error(); const data=await r.json(); list.replaceChildren();
      if(!data.items.length){const p=document.createElement("p");p.className="empty";p.textContent=l.empty;list.append(p);return}
      for(const item of data.items){const article=document.createElement("article");article.className="comment";const meta=document.createElement("div");meta.className="meta";const name=document.createElement("strong");name.textContent=item.author.name||l.guest;meta.append(name);if(!item.author.registered){const badge=document.createElement("span");badge.className="badge";badge.textContent=l.guest;meta.append(badge)}const time=document.createElement("time");time.dateTime=item.created_at;time.textContent=new Intl.DateTimeFormat(undefined,{dateStyle:"medium"}).format(new Date(item.created_at));meta.append(time);const body=document.createElement("p");body.textContent=item.body;article.append(meta,body);list.append(article)}
    } catch { list.innerHTML=`<p class="empty">${l.error}</p>`; }
  }
  async submit(event) {
    event.preventDefault();const form=event.currentTarget,button=form.querySelector("button"),out=form.querySelector("output"),data=new FormData(form),l=this.labels;button.disabled=true;button.textContent=l.sending;out.textContent="";
    try{const r=await fetch(`${API}/v1/comments`,{method:"POST",headers:this.headers(),body:JSON.stringify({resource:this.resource,body:String(data.get("body")||""),guest_name:String(data.get("name")||""),guest_email:String(data.get("email")||"")})});if(!r.ok){const p=await r.json().catch(()=>({}));throw new Error(p?.error?.message||l.error)}form.reset();out.textContent=l.thanks;await this.load()}catch(e){out.textContent=e.message||l.error}finally{button.disabled=false;button.textContent=l.send}
  }
}

if (!customElements.get("integ-comments")) customElements.define("integ-comments", IntegComments);
