// Run with jsdom available on NODE_PATH, like ui-regression.cjs.
const {JSDOM}=require('jsdom');
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),path=require('node:path');
const base=path.resolve(__dirname,'../internal/httpapi/static');
const dom=new JSDOM(fs.readFileSync(path.join(base,'index.html'),'utf8'),{url:'http://localhost/',runScripts:'outside-only'}),w=dom.window;
w.HTMLDialogElement.prototype.showModal=function(){this.open=true};w.HTMLDialogElement.prototype.close=function(){this.open=false};
w.URL.createObjectURL=f=>'blob:'+f.name;w.URL.revokeObjectURL=()=>{};
let created=false,approved=false,proposalBody;const writes=[];let gameMode=false;const consoleOptions=["PS5"];
const job={id:'job-1',item_id:'',category:'Amiibo',state:'review',input:{text:''},photos:[{id:'photo-1',filename:'mario.png'}],result:{title:'Mario — Super Smash Bros. amiibo',outcome:'confident',matches:[{candidate:{provider:'amiibo',id:'0000000000000002',label:'Mario'},role:'alternative',confidence:'confident',reason:'The flame pose and dark base match.'}]}};
w.fetch=async(url,options={})=>{let data;
 if(url==='/api/config')data={config:{model_configured:true,image_storage_configured:true,vision_model_configured:true}};
 else if(url==='/api/vocabulary')data={fields:[...(gameMode?[{key:'console',label:'Console',data_type:'enum',config:{options:[...consoleOptions]}}]:[]),{key:'title',label:'Title',data_type:'text'},{key:'location',label:'Location',data_type:'text'}],tag_fields:{},templates:['Amiibo','Vinyl','Movies','Video games','Hardware','Other'].map((name,index)=>({id:'type-'+index,name,description:name==='Amiibo'?'Nintendo figures and cards':'Your configured item type',default_tags:[name],default_values:{},attributes:[],intake_config:{providers:['amiibo']}}))};
 else if(url==='/api/library')data={total:0,tags:[]};
 else if(url==='/api/items/owned-1')data={id:'owned-1',title:proposalBody?.attributes?.title||'Mario'};
 else if(url.startsWith('/api/items?'))data={items:[],total:0};
 else if(url.startsWith('/api/proposals?'))data={proposals:[]};
 else if(url==='/api/intake'&&options.method==='POST'){created=true;writes.push('upload');data={id:job.id};assert.equal(options.body.get('category'),'Amiibo');assert.equal(options.body.getAll('images').length,1)}
 else if(url==='/api/intake/job-1?compact=1')data={...job,state:approved?'approved':job.state,item_id:approved?'owned-1':''};
 else if(url==='/api/intake/job-1/cards')data={cards:[{title:gameMode?'':'Mario',subtitle:'Super Smash Bros.',facts:[{label:'Game series',value:'Super Mario'},{label:'Type',value:'Figure'}],source:'AmiiboAPI',url:'https://amiiboapi.org/api/amiibo/?id=0000000000000002',layout:'identity'},...(job.result.matches.length>1?[{title:'Mario',subtitle:'Super Mario Bros.',facts:[{label:'Type',value:'Figure'}],source:'AmiiboAPI',url:'https://amiiboapi.org/api/amiibo/?id=0000000000340102'}]:[])]};
 else if(url==='/api/intake/job-1/ownership')data={ownership:[]};
 else if(url==='/api/proposals/graph'){writes.push('vocabulary-proposal');assert.equal(JSON.parse(options.body).operation,'extend_enum');data={id:'console-proposal'}}
 else if(url==='/api/proposals/console-proposal/decision'){writes.push('vocabulary-approve');consoleOptions.push('Nintendo Switch 2');data={status:'approved'}}
 else if(url==='/api/intake/job-1/proposal'){writes.push('proposal');proposalBody=JSON.parse(options.body);data={id:'proposal-1'}}
 else if(url==='/api/proposals/proposal-1/decision'){writes.push('approve');assert.equal(JSON.parse(options.body).approve,true);approved=true;data={status:'approved'}}
 else throw Error('Unexpected API: '+url);
 return{ok:true,json:async()=>data};
};
for(const file of ['app.js','library.js','intake.js','navigation.js'])new vm.Script(fs.readFileSync(path.join(base,file),'utf8')).runInContext(dom.getInternalVMContext());
const call=code=>vm.runInContext(code,dom.getInternalVMContext());
const wait=()=>new Promise(resolve=>setTimeout(resolve,10));
const click=text=>{const b=[...w.document.querySelectorAll('button')].find(b=>b.textContent.trim()===text);assert(b,'Button missing: '+text);b.click()};
(async()=>{
 await wait();call('showCapture()');await wait();assert.equal(w.document.querySelectorAll('.kind-choice').length,6);
 w.document.querySelector('.kind-choice').click();assert(w.document.querySelector('.flow-progress .current').textContent.includes('Photos'));
 const input=w.document.querySelector('input[aria-label="Choose item photos"]');Object.defineProperty(input,'files',{value:[new w.File(['test'],'mario.png',{type:'image/png'})],configurable:true});input.onchange();assert.equal(w.document.querySelectorAll('.capture-thumb').length,1);
 click('← Change type');await wait();w.document.querySelector('.kind-choice').click();assert.equal(w.document.querySelectorAll('.capture-thumb').length,1,'Back must retain selected photos');
 click('Find this item →');await wait();await wait();assert(created);assert.equal(w.document.querySelectorAll('.identity-facts dd').length,2);assert(!w.document.querySelector('.flow-screen').textContent.includes('0000000000000002'),'Technical IDs should not be review content');assert.deepEqual(writes,['upload'],'Identification must not create an owned item');
 click('Use this match →');assert(w.document.querySelector('.flow-progress .current').textContent.includes('Save'));assert.deepEqual(writes,['upload']);
 const picker=w.document.querySelector('.personal-notes select');assert(picker);assert(![...picker.options].some(o=>o.value==='title'),'Personal fields must not offer a conflicting second name');picker.value='location';picker.onchange();w.document.querySelector('[aria-label="Location"]').value='Display shelf';
 click('Add to my inventory');await wait();await wait();assert(approved);assert.deepEqual(proposalBody.selected,[0]);assert.deepEqual(Object.keys(proposalBody.attributes).sort(),['location','title']);assert.equal(proposalBody.attributes.location,'Display shelf');assert.deepEqual(writes,['upload','proposal','approve']);assert(w.document.querySelector('.flow-heading').textContent.includes('Saved to your collection'));
 click('Add another');assert(w.document.querySelector('.flow-progress .current').textContent.includes('Photos'));assert.equal(w.document.querySelectorAll('.capture-thumb').length,0);
 approved=false;job.result.matches.push({candidate:{provider:'amiibo',id:'0000000000340102',label:'Mario'},confidence:'rejected',role:'alternative',reason:'Another version was suggested.'});await call("openIdentification('job-1')");await wait();
 const next=w.document.querySelector('[aria-label="Next version"]');assert(next,'Human must be able to inspect the alternative');next.click();click('Use this match →');assert(w.document.querySelector('[aria-label="Item name"]').value.includes('Super Mario Bros.'),'Name must follow the human-selected series');click('Add to my inventory');await wait();await wait();assert.equal(proposalBody.confirm_alternative,true);assert.deepEqual(proposalBody.selected,[1]);
 approved=false;gameMode=true;job.input.field_keys=['title','console'];job.input.default_tags=['Games','Consoles','Games','Consoles'];job.result.inferred={title:'Among Us: Crewmate Edition'};job.result.matches=job.result.matches.slice(0,1);job.result.matches[0].candidate.label='Among Us';job.result.title='Among Us — Nintendo Switch 2';
 await call('loadLibraryMeta()');await call("openIdentification('job-1')");await wait();assert.equal(w.document.querySelector('.identity-card h3').textContent,'Among Us','Empty source labels must retain the search title');
 click('Use this match →');const count=writes.length;click('Add to my inventory');await wait();assert.equal(writes.length,count,'Unconfirmed console must not create proposals');
 const consoleInput=w.document.querySelector('[aria-label="Console"]');consoleInput.value='Nintendo Switch 2';consoleInput.oninput();assert(w.document.querySelector('#console-choices option[value="Nintendo Switch"]'));assert(w.document.querySelector('#console-choices option[value="Nintendo Switch 2"]'));
 click('Approve new console choice');await wait();await wait();assert.deepEqual(writes.slice(count),['vocabulary-proposal','vocabulary-approve'],'Vocabulary approval must not save an owned item');
 w.document.querySelector('.identity-card input[type="checkbox"]').checked=true;click('Add to my inventory');await wait();await wait();assert.equal(proposalBody.attributes.console,'Nintendo Switch 2');assert.equal(proposalBody.attributes.title,'Among Us: Crewmate Edition');assert.deepEqual(proposalBody.tags,['Games','Nintendo Switch 2']);
 approved=false;job.result.matches[0].candidate.label='Different Game';await call("openIdentification('job-1')");await wait();click('Use this match →');assert(w.document.querySelector('[aria-label="Item name"]').value.startsWith('Different Game'),'Changing the selected game must not reuse another game’s inferred edition');
 approved=false;job.state='queued';let processed;w.htmx={process:target=>processed=target};await call("openIdentification('job-1')");assert(processed?.getAttribute('hx-get').endsWith('/status'));const photo=w.document.querySelector('.flow-picture');await wait();assert.equal(w.document.querySelector('.flow-picture'),photo,'Waiting must retain its photo');job.state='review';w.document.body.dispatchEvent(new w.CustomEvent('identificationReady',{detail:{id:'job-1'}}));await wait();await wait();assert(w.document.querySelector('.identity-card h3'),'Server completion must advance into review');
 console.log('PASS: photo capture, human match correction, title fallback, console confirmation, approved vocabulary expansion, and saved console.');w.close();
})().catch(e=>{console.error(e);w.close();process.exit(1)});
