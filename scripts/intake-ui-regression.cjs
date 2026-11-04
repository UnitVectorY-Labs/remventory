// Run with jsdom available on NODE_PATH, like ui-regression.cjs.
const {JSDOM}=require('jsdom');
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),path=require('node:path');
const base=path.resolve(__dirname,'../internal/httpapi/static');
const dom=new JSDOM(fs.readFileSync(path.join(base,'index.html'),'utf8'),{url:'http://localhost/',runScripts:'outside-only'}),w=dom.window;
w.HTMLDialogElement.prototype.showModal=function(){this.open=true};w.HTMLDialogElement.prototype.close=function(){this.open=false};
w.URL.createObjectURL=f=>'blob:'+f.name;w.URL.revokeObjectURL=()=>{};
let created=false,approved=false,proposalBody;const writes=[];
const job={id:'job-1',item_id:'',category:'Amiibo',state:'review',input:{text:''},photos:[{id:'photo-1',filename:'mario.png'}],result:{title:'Mario — Super Smash Bros. amiibo',outcome:'confident',matches:[{candidate:{provider:'amiibo',id:'0000000000000002',label:'Mario'},role:'alternative',confidence:'confident',reason:'The flame pose and dark base match.'}]}};
w.fetch=async(url,options={})=>{let data;
 if(url==='/api/config')data={config:{model_configured:true,image_storage_configured:true,vision_model_configured:true}};
 else if(url==='/api/vocabulary')data={fields:[{key:'title',label:'Title',data_type:'text'}],tag_fields:{},templates:[]};
 else if(url==='/api/library')data={total:0,tags:[]};
 else if(url==='/api/items/owned-1')data={id:'owned-1',title:proposalBody?.attributes?.title||'Mario'};
 else if(url.startsWith('/api/items?'))data={items:[],total:0};
 else if(url.startsWith('/api/proposals?'))data={proposals:[]};
 else if(url==='/api/intake'&&options.method==='POST'){created=true;writes.push('upload');data={id:job.id};assert.equal(options.body.get('category'),'Amiibo');assert.equal(options.body.getAll('images').length,1)}
 else if(url==='/api/intake/job-1?compact=1')data={...job,state:approved?'approved':'review',item_id:approved?'owned-1':''};
 else if(url==='/api/intake/job-1/cards')data={cards:[{title:'Mario',subtitle:'Super Smash Bros.',facts:[{label:'Game series',value:'Super Mario'},{label:'Type',value:'Figure'}],source:'AmiiboAPI',url:'https://amiiboapi.org/api/amiibo/?id=0000000000000002',layout:'identity'},...(job.result.matches.length>1?[{title:'Mario',subtitle:'Super Mario Bros.',facts:[{label:'Type',value:'Figure'}],source:'AmiiboAPI',url:'https://amiiboapi.org/api/amiibo/?id=0000000000340102'}]:[])]};
 else if(url==='/api/intake/job-1/ownership')data={ownership:[]};
 else if(url==='/api/intake/job-1/proposal'){writes.push('proposal');proposalBody=JSON.parse(options.body);data={id:'proposal-1'}}
 else if(url==='/api/proposals/proposal-1/decision'){writes.push('approve');assert.equal(JSON.parse(options.body).approve,true);approved=true;data={status:'approved'}}
 else throw Error('Unexpected API: '+url);
 return{ok:true,json:async()=>data};
};
for(const file of ['app.js','library.js','intake.js'])new vm.Script(fs.readFileSync(path.join(base,file),'utf8')).runInContext(dom.getInternalVMContext());
const call=code=>vm.runInContext(code,dom.getInternalVMContext());
const wait=()=>new Promise(resolve=>setTimeout(resolve,10));
const click=text=>{const b=[...w.document.querySelectorAll('button')].find(b=>b.textContent.trim()===text);assert(b,'Button missing: '+text);b.click()};
(async()=>{
 await wait();call('showCapture()');assert.equal(w.document.querySelectorAll('.kind-choice').length,6);
 w.document.querySelector('.kind-choice').click();assert(w.document.querySelector('.flow-progress .current').textContent.includes('Photos'));
 const input=w.document.querySelector('input[aria-label="Choose item photos"]');Object.defineProperty(input,'files',{value:[new w.File(['test'],'mario.png',{type:'image/png'})],configurable:true});input.onchange();assert.equal(w.document.querySelectorAll('.capture-thumb').length,1);
 click('← Change type');w.document.querySelector('.kind-choice').click();assert.equal(w.document.querySelectorAll('.capture-thumb').length,1,'Back must retain selected photos');
 click('Find this item →');await wait();await wait();assert(created);assert.equal(w.document.querySelectorAll('.identity-facts dd').length,2);assert(!w.document.querySelector('.flow-screen').textContent.includes('0000000000000002'),'Technical IDs should not be review content');assert.deepEqual(writes,['upload'],'Identification must not create an owned item');
 click('Use this match →');assert(w.document.querySelector('.flow-progress .current').textContent.includes('Save'));assert.deepEqual(writes,['upload']);
 click('Add to my inventory');await wait();await wait();assert(approved);assert.deepEqual(proposalBody.selected,[0]);assert.deepEqual(Object.keys(proposalBody.attributes),['title']);assert.deepEqual(writes,['upload','proposal','approve']);assert(w.document.querySelector('.flow-heading').textContent.includes('Saved to your collection'));
 click('Add another Amiibo');assert(w.document.querySelector('.flow-progress .current').textContent.includes('Photos'));assert.equal(w.document.querySelectorAll('.capture-thumb').length,0);
 approved=false;job.result.matches.push({candidate:{provider:'amiibo',id:'0000000000340102',label:'Mario'},confidence:'rejected',role:'alternative',reason:'Another version was suggested.'});await call("openIdentification('job-1')");await wait();
 const next=w.document.querySelector('[aria-label="Next version"]');assert(next,'Human must be able to inspect the alternative');next.click();click('Use this match →');assert(w.document.querySelector('[aria-label="Item name"]').value.includes('Super Mario Bros.'),'Name must follow the human-selected series');click('Add to my inventory');await wait();await wait();assert.equal(proposalBody.confirm_alternative,true);assert.deepEqual(proposalBody.selected,[1]);
 console.log('PASS: progressive capture, retained photos, minimal review, explicit save, and human correction of an automated choice.');w.close();
})().catch(e=>{console.error(e);w.close();process.exit(1)});
