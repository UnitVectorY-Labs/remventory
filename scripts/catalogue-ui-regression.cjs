// Run with jsdom on NODE_PATH; all data and authority responses are synthetic.
const {JSDOM}=require('jsdom'),{readFileSync}=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),path=require('node:path');
const base=path.resolve(__dirname,'../internal/httpapi/static'),dom=new JSDOM(readFileSync(path.join(base,'index.html'),'utf8'),{url:'http://localhost/',runScripts:'outside-only'}),w=dom.window;
w.HTMLDialogElement.prototype.showModal=function(){this.open=true};w.HTMLDialogElement.prototype.close=function(){this.open=false};
const item={id:'owned-item',title:'My physical copy',tags:['Custom type'],attributes:{title:'My physical copy',year:1999,condition:'Mine',links:[]}},profile={id:'type-1',name:'Custom type',default_tags:['Custom type'],default_values:{},attributes:[{key:'condition'}],intake_config:{providers:['wikidata'],prompt:'Use the visible title',display_fields:{}}};
let linked=true,seq=0;const proposals=new Map(),writes=[];
const card={title:'The Matrix',source:'Wikidata',url:'https://www.wikidata.org/wiki/Q8337',facts:[{label:'Original release',value:'1999'}],entity_id:'entity-1'};
w.fetch=async(url,options={})=>{let data={};
 if(url==='/api/config')data={config:{model_configured:true,image_storage_configured:true,vision_model_configured:true}};
 else if(url==='/api/vocabulary')data={fields:[{key:'title',label:'Title',data_type:'text'},{key:'condition',label:'Condition',data_type:'text'},{key:'year',label:'Year',data_type:'number'},{key:'links',label:'Links',data_type:'links'}],templates:[profile],tag_fields:{}};
 else if(url==='/api/library')data={tags:[{name:'Custom type',count:1}],total:1};
 else if(url.startsWith('/api/items?'))data={items:[item],total:1,identification_status:{[item.id]:linked?'identified':'unidentified'}};
 else if(url.startsWith('/api/proposals?'))data={proposals:[]};
 else if(url==='/api/items/owned-item')data=item;
 else if(url==='/api/items/owned-item/cards')data={cards:linked?[card]:[]};
 else if(url==='/api/items/owned-item/canonical')data={links:linked?[{id:'entity-1',label:'The Matrix',provider:'wikidata',relation:'identified_as'}]:[]};
 else if(url==='/api/items/owned-item/catalogue/proposal'){
  const input=JSON.parse(options.body),id='proposal-'+(++seq);writes.push(input);const p={id,type:input.operation.startsWith('reference')?'item_change':'catalogue_change',status:'pending',proposed_payload:{...input,item_id:item.id,label:'The Matrix'}};proposals.set(id,p);data={proposal:p,card:input.operation==='link'?card:null,reference:input.operation.startsWith('reference')?{name:input.name,url:input.url,operation:input.operation}:null};
 }
 else if(url.startsWith('/api/proposals/')){const id=url.split('/')[3],p=proposals.get(id);assert(p);if(options.method==='POST'){const decision=JSON.parse(options.body);p.status=decision.approve?'approved':'rejected';if(decision.approve){const v=p.proposed_payload;if(v.operation==='unlink')linked=false;if(v.operation==='link')linked=true;if(v.operation==='reference')item.attributes.links.push({name:v.name,url:v.url});if(v.operation==='reference_remove')item.attributes.links=item.attributes.links.filter(l=>l.url!==v.url);}}data=p;}
 else throw Error('Unexpected API '+url);
 return{ok:true,json:async()=>JSON.parse(JSON.stringify(data))};
};
for(const file of ['app.js','library.js','intake.js','catalogue.js','navigation.js'])new vm.Script(readFileSync(path.join(base,file),'utf8')).runInContext(dom.getInternalVMContext());
const call=code=>vm.runInContext(code,dom.getInternalVMContext()),wait=()=>new Promise(r=>setTimeout(r,15)),click=text=>{const b=[...w.document.querySelectorAll('#catalog-modal button')].find(b=>b.textContent.trim()===text);assert(b,'Missing '+text);b.click();};
(async()=>{
 await wait();assert(w.document.querySelector('#tag-filters').textContent.includes('Custom type'),'Navigation must use runtime types');
 await call('showCapture()');await wait();assert.equal(w.document.querySelectorAll('.kind-choice').length,1);assert(w.document.querySelector('.kind-choice').textContent.includes('Custom type'));
 call(`setWorkspace('collection');openItemDetail(${JSON.stringify(item)}, {})`);await wait();assert(w.document.querySelector('.owner-fields').textContent.includes('1999'));assert.equal(w.document.querySelector('.owner-fields details'),null,'Personal values must not be hidden in a fold');
 await call(`manageCatalogue(${JSON.stringify(item)})`);click('Unlink');await wait();assert(linked,'Unlink must require confirmation');click('Confirm');await wait();await wait();assert(!linked);assert.equal(item.attributes.condition,'Mine');assert.equal(item.attributes.year,1999);
 await call(`manageCatalogue(${JSON.stringify(item)})`);const form=w.document.querySelector('.manual-url-form');form.querySelector('input[type=url]').value='https://www.wikidata.org/wiki/Q8337';form.onsubmit({preventDefault(){}});await wait();assert(!linked);click('Confirm');await wait();await wait();assert(linked,'Manual URL should not require an AI job');
 await call(`manageCatalogue(${JSON.stringify(item)})`);const refForm=w.document.querySelector('.manual-url-form'),kind=refForm.querySelector('select');kind.value='reference';kind.onchange();refForm.querySelector('input[type=url]').value='https://example.com/my-copy';refForm.querySelector('input[placeholder="Link name"]').value='My reference';refForm.onsubmit({preventDefault(){}});await wait();assert.equal(item.attributes.links.length,0);click('Confirm');await wait();await wait();assert.equal(item.attributes.links.length,1);assert.equal(writes.at(-1).operation,'reference');assert.equal(item.attributes.condition,'Mine');
 console.log('PASS: runtime type navigation/capture, visible owner fields, manual canonical URL approval, unlink approval and generic references.');w.close();
})().catch(e=>{console.error(e);w.close();process.exit(1)});
