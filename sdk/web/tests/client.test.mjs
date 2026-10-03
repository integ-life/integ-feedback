import test from 'node:test';
import assert from 'node:assert/strict';
import { FeedbackClient, prepareFeedbackImage } from '../dist/index.js';

test('shared client posts consented image reports and remains compatible with text feedback', async () => {
  const previous=globalThis.fetch; const calls=[];
  globalThis.fetch=async(url,init)=>{calls.push({url,init});return new Response(JSON.stringify({id:'report-1',has_attachment:true}),{status:201});};
  try {
    const client=new FeedbackClient({apiUrl:'https://discuss.integ.life/',projectKey:'pk_test'});
    const image={base64:'YWJj',width:3,height:3,bytes:3};
    await assert.rejects(client.submitFeedback({resource:'tool:test',kind:'issue',body:'test',image}),/consent/i);
    assert.equal(calls.length,0);
    const receipt=await client.submitFeedback({resource:'tool:test',kind:'issue',body:'test',image,attachmentConsent:true});
    assert.equal(receipt.id,'report-1');
    assert.equal(calls[0].url,'https://discuss.integ.life/v1/feedback');
    assert.equal(calls[0].init.headers['X-Project-Key'],'pk_test');
    assert.equal(JSON.parse(calls[0].init.body).attachment_consent,true);
    assert.equal(JSON.parse(calls[0].init.body).image_base64,'YWJj');
    await client.submitFeedback({resource:'tool:test',kind:'question',body:'text'});
    assert.equal(JSON.parse(calls[1].init.body).image_base64,undefined);
  } finally { globalThis.fetch=previous; }
});

test('image preparation is local, bounded, opaque JPEG and releases the bitmap', async () => {
  const previous={fetch:globalThis.fetch,createImageBitmap:globalThis.createImageBitmap,document:globalThis.document,FileReader:globalThis.FileReader};
  let closed=0; const draws=[];
  globalThis.fetch=()=>{throw new Error('preparation must not upload');};
  globalThis.createImageBitmap=async()=>({width:4000,height:3000,close(){closed++;}});
  const canvas={width:0,height:0,getContext:()=>({fillRect(){},drawImage(...args){draws.push(args);}}),toBlob(callback,type){assert.equal(type,'image/jpeg');callback(new Blob(['jpeg'],{type}));}};
  globalThis.document={createElement:()=>canvas};
  globalThis.FileReader=class{readAsDataURL(blob){blob.text().then(text=>{this.result='data:image/jpeg;base64,'+Buffer.from(text).toString('base64');this.onload();});}};
  try {
    await assert.rejects(prepareFeedbackImage(new Blob(['x'],{type:'image/svg+xml'})),/JPEG/);
    const image=await prepareFeedbackImage(new Blob(['photo'],{type:'image/png'}));
    assert.equal(image.width,1280);assert.equal(image.height,960);assert.equal(image.bytes,4);assert.equal(closed,1);assert.equal(draws.length,1);
    assert.equal(image.base64,'anBlZw==');
  } finally { Object.assign(globalThis,previous); }
});
