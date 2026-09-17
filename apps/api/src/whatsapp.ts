import { randomUUID } from 'node:crypto';
import type { Sql,Scope } from './db';
import { one } from './db';
import { Core } from './core';
import { hash,seal,unseal } from './security';
export class WhatsAppFlow {
 constructor(private core:Core,private key:string){}
 async receive(q:Sql,s:Scope,messageId:string,sender:string,message:string):Promise<string>{
  await q.query('INSERT INTO whatsapp_messages(company_id,message_id,sender) VALUES($1,$2,$3) ON CONFLICT DO NOTHING',[s.company,messageId,sender]);
  const msg=await one(q,'SELECT * FROM whatsapp_messages WHERE company_id=$1 AND message_id=$2 FOR UPDATE',[s.company,messageId]);
  if(msg.sender!==sender)throw new Error('Message identifier collision across senders');
  if(msg.response)return unseal(msg.response,this.key);
  const finish=async(response:string)=>{await q.query('UPDATE whatsapp_messages SET response=$1 WHERE company_id=$2 AND message_id=$3',[seal(response,this.key),s.company,messageId]);return response;};
  const body=message.trim();
  if(body.toLowerCase().startsWith('vincular ')){
   const link=await one(q,'UPDATE phone_links SET used=true WHERE company_id=$1 AND code_hash=$2 AND NOT used AND expires_at>now() RETURNING subject',[s.company,hash(body.slice(9).trim())]);
   if(!link)return finish('Código inválido ou expirado. Gere outro código na sua conta.');
   await q.query("UPDATE memberships SET whatsapp=$1,whatsapp_verified=true WHERE company_id=$2 AND subject=$3 AND role='resident' AND active",[sender,s.company,link.subject]);
   await this.core.event(q,s,'whatsapp.linked',undefined,undefined,{subject:link.subject});
   return finish('WhatsApp vinculado. Envie CONVITE para cadastrar uma visita.');
  }
  const member=await one(q,'SELECT * FROM app.whatsapp_member($1)',[sender]);
  if(!member)return finish('Este número não está vinculado a um morador. Entre no app e use a opção Vincular WhatsApp.');
  const resident:Scope={subject:member.subject,company:s.company,role:'resident',condos:member.condo_ids,units:member.unit_ids};
  await q.query('INSERT INTO whatsapp_flows(company_id,sender,step) VALUES($1,$2,$3) ON CONFLICT DO NOTHING',[s.company,sender,'idle']);
  let flow=await one(q,'SELECT * FROM whatsapp_flows WHERE company_id=$1 AND sender=$2 FOR UPDATE',[s.company,sender]);
  if(new Date(flow.updated_at).getTime()<Date.now()-15*60000)flow={step:'idle',data:{}};
  const set=async(step:string,data:any,response:string)=>{await q.query('UPDATE whatsapp_flows SET step=$1,data=$2,updated_at=now() WHERE company_id=$3 AND sender=$4',[step,data,s.company,sender]);return finish(response);};
  if(body.toLowerCase()==='cancelar')return set('idle',{},'Cadastro interrompido. Envie CONVITE para começar novamente.');
  if(body.toLowerCase()==='convite'||flow.step==='idle'){
   const units=(await q.query("SELECT id,condo_id,data FROM records WHERE kind='units' AND id=ANY($1::uuid[]) AND condo_id=ANY($2::uuid[])",[member.unit_ids,member.condo_ids])).rows;
   if(!units.length)return finish('Sua conta ainda não tem uma unidade vinculada. Entre em contato com a administração.');
   return set('unit',{units},`Escolha a unidade pelo número:\n${units.map((u:any,i:number)=>`${i+1}. ${u.data.name}`).join('\n')}\nEnvie CANCELAR a qualquer momento.`);
  }
  const data=flow.data;
  if(flow.step==='unit'){const unit=data.units[Number(body)-1];if(!unit||!/^[1-9]\d*$/.test(body))return finish('Responda com o número de uma das unidades.');return set('name',{unit},'Qual é o nome completo do convidado?');}
  if(flow.step==='name'){if(body.length<3||body.length>200)return finish('Informe um nome entre 3 e 200 caracteres.');return set('start',{...data,guest_name:body},'Informe o início da visita no formato DD/MM/AAAA HH:MM (horário de Brasília).');}
  if(flow.step==='start'||flow.step==='end'){
   const match=body.match(/^(\d{2})\/(\d{2})\/(\d{4}) (\d{2}):(\d{2})$/);if(!match)return finish('Use DD/MM/AAAA HH:MM.');
   const value=`${match[3]}-${match[2]}-${match[1]}T${match[4]}:${match[5]}:00-03:00`;
   const d=new Date(value);if(!Number.isFinite(d.getTime())||d.toLocaleDateString('pt-BR',{timeZone:'America/Sao_Paulo'})!==`${match[1]}/${match[2]}/${match[3]}`)return finish('A data não é válida.');
   if(flow.step==='start')return set('end',{...data,valid_from:d.toISOString()},'Informe o fim da visita no mesmo formato.');
   if(d.getTime()<=Date.now()||d<=new Date(data.valid_from)||d.getTime()-new Date(data.valid_from).getTime()>7*86400000)return finish('O término deve ser posterior ao início, com duração de até sete dias.');
   return set('plate',{...data,valid_until:d.toISOString()},'Informe a placa do veículo, ou SEM VEÍCULO.');
  }
  if(flow.step==='plate'){
   const plate=body.toUpperCase();if(plate!=='SEM VEÍCULO'&&!/^[A-Z0-9-]{5,10}$/.test(plate))return finish('Informe uma placa válida ou SEM VEÍCULO.');
   const accesses=(await q.query("SELECT id,data FROM records WHERE kind='access-points' AND condo_id=$1",[data.unit.condo_id])).rows;
   if(!accesses.length)return set('idle',{},'Ainda não há acesso configurado para esta unidade.');
   return set('access',{...data,vehicle_plate:plate==='SEM VEÍCULO'?undefined:plate,accesses},`Escolha o acesso:\n${accesses.map((a:any,i:number)=>`${i+1}. ${a.data.name}`).join('\n')}`);
  }
  if(flow.step==='access'){const a=data.accesses[Number(body)-1];if(!a)return finish('Informe o número do acesso.');return set('confirm',{...data,access_id:a.id},`Confirme a visita:\n${data.guest_name}\nUnidade ${data.unit.data.name}\nAcesso ${a.data.name}\nDe ${new Date(data.valid_from).toLocaleString('pt-BR',{timeZone:'America/Sao_Paulo'})} até ${new Date(data.valid_until).toLocaleString('pt-BR',{timeZone:'America/Sao_Paulo'})}\nResponda CONFIRMAR ou CANCELAR.`);}
  if(flow.step==='confirm'){
   if(body.toLowerCase()!=='confirmar')return finish('Responda CONFIRMAR para criar o convite ou CANCELAR.');
   // Switch the transaction to the resident's scope before invoking the shared authorization engine.
   await q.query("SELECT set_config('app.role','resident',true),set_config('app.subject',$1,true),set_config('app.condos',$2,true),set_config('app.units',$3,true)",[resident.subject,resident.condos!.join(','),resident.units!.join(',')]);
   const invite=await this.core.invite(q,resident,{unit_id:data.unit.id,guest_name:data.guest_name,vehicle_plate:data.vehicle_plate,valid_from:data.valid_from,valid_until:data.valid_until,access_ids:[data.access_id],request_key:`whatsapp-${randomUUID()}`,channel:'whatsapp'});
   return set('idle',{},`Convite criado. Compartilhe este link com o convidado:\n${invite.url}`);
  }
  return set('idle',{},'Envie CONVITE para cadastrar uma visita.');
 }
}
