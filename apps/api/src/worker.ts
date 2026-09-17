import { Queue,Worker } from 'bullmq';
import { db,env,core } from './runtime';
import { verifyDomainTLS } from './domain-tls';
const url=new URL(env.REDIS_URL||'redis://localhost:6379');
const connection={host:url.hostname,port:Number(url.port||6379),password:url.password||undefined,...(url.protocol==='rediss:'?{tls:{}}:{})};
const queue=new Queue('portaria-maintenance',{connection});
async function main(){
 await queue.upsertJobScheduler('periodic-maintenance',{every:30000},{name:'scan',data:{},opts:{removeOnComplete:100,removeOnFail:100}});
 const worker=new Worker('portaria-maintenance',async()=>{
  const companies=await db.transaction({subject:'maintenance',platform:true},q=>q.query('SELECT id FROM companies').then(r=>r.rows));
  for(const company of companies){const s={subject:'maintenance',company:company.id,role:'support'};await db.transaction(s,async q=>{
   const expired=(await q.query("UPDATE commands SET state=CASE WHEN state='sent' THEN 'unknown' ELSE 'expired' END WHERE state IN ('pending','sent') AND expires_at<now() RETURNING id,condo_id,state")).rows;
   for(const c of expired)await core.event(q,s,`command.${c.state}`,c.id,c.condo_id);
   await q.query("DELETE FROM phone_links WHERE expires_at<now()-interval '1 day'");
   await q.query("DELETE FROM whatsapp_flows WHERE updated_at<now()-interval '1 day'");
   await q.query("DELETE FROM whatsapp_messages WHERE created_at<now()-interval '7 days'");
   await q.query("UPDATE integrations SET status='disconnected' WHERE kind='whatsapp' AND status='connected' AND updated_at<now()-interval '2 minutes'");
   const offline=(await q.query("SELECT id,condo_id FROM agent_keys WHERE active AND last_seen<now()-interval '60 seconds' AND NOT EXISTS(SELECT 1 FROM records WHERE kind='incidents' AND data->>'agent_id'=agent_keys.id::text AND data->>'status'='open')")).rows;
   for(const a of offline)await q.query("INSERT INTO records(company_id,condo_id,kind,data) VALUES($1,$2,'incidents',$3)",[s.company,a.condo_id,{name:'Agente desconectado',description:'Verifique a conexão do condomínio.',severity:'critical',status:'open',agent_id:a.id}]);
   await q.query("UPDATE records SET data=jsonb_set(data,'{status}','\"closed\"') WHERE kind='incidents' AND data->>'agent_id' IN (SELECT id::text FROM agent_keys WHERE last_seen>now()-interval '60 seconds') AND data->>'status'='open'");
   await q.query('INSERT INTO maintenance_runs(company_id) VALUES($1) ON CONFLICT(company_id) DO UPDATE SET last_run=now()',[s.company]);
  });
  if(env.DOMAIN_TARGET){const domains=await db.transaction(s,q=>q.query('SELECT id,hostname FROM domains WHERE verified_at IS NOT NULL AND tls_checked_at IS NULL').then(r=>r.rows));for(const d of domains){let ok=false;try{ok=await verifyDomainTLS(d.hostname,env.DOMAIN_TARGET);}catch{}if(ok)await db.transaction(s,async q=>{await q.query('UPDATE domains SET tls_checked_at=now() WHERE id=$1',[d.id]);await core.event(q,s,'domain.tls_verified',d.id);});}}
  }
 },{connection,concurrency:1});
 worker.on('failed',(_job,error)=>console.error('maintenance_failed',error.name));
 async function close(){await worker.close();await queue.close();await db.pool.end();process.exit(0);}process.on('SIGTERM',close);process.on('SIGINT',close);
}
main().catch(e=>{console.error('worker_start_failed',e.message);process.exitCode=1;});
