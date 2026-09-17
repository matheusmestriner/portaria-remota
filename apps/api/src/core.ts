import { BadRequestException,ConflictException,ForbiddenException,NotFoundException,ServiceUnavailableException } from '@nestjs/common';
import { randomInt } from 'node:crypto';
import { one,type Scope,type Sql } from './db';
import { hash,seal,token,unseal,requireRole } from './security';
import { invitationSchema,recordSchemas,openSchema } from './schemas';
export const admins=['company_admin','support'];
export const operators=[...admins,'supervisor','operator'];
export class Core {
 constructor(private key:string,private publicUrl:string){}
 async event(q:Sql,s:Scope,kind:string,resource?:string,condo?:string,detail:any={},unit?:string){await q.query('INSERT INTO events(company_id,condo_id,unit_id,kind,actor,resource_id,detail) VALUES($1,$2,$3,$4,$5,$6,$7)',[s.company,condo||null,unit||null,kind,s.subject,resource||null,detail]);}
 async active(q:Sql,s:Scope){const c=await one(q,'SELECT status FROM companies WHERE id=$1',[s.company]);if(c?.status!=='active')throw new ConflictException('A empresa ainda não está ativa. Conclua a configuração.');}
 async record(q:Sql,id:string,kind?:string){const r=await one(q,'SELECT * FROM records WHERE id=$1',[id]);if(!r||kind&&r.kind!==kind)throw new NotFoundException('Cadastro não encontrado.');return r;}
 async createRecord(q:Sql,s:Scope,kind:string,body:any){
  requireRole(s,kind==='incidents'?operators:['people','vehicles'].includes(kind)?[...admins,'manager','resident']:['blocks','units','rules','credentials'].includes(kind)?[...admins,'manager']:admins);
  if(!recordSchemas[kind])throw new NotFoundException();
  const data:any=recordSchemas[kind].parse(body.data);const condo=await one(q,'SELECT id FROM condos WHERE id=$1',[body.condo_id]);if(!condo)throw new NotFoundException('Condomínio não encontrado.');
  if(body.unit_id){const unit=await this.record(q,body.unit_id,'units');if(unit.condo_id!==body.condo_id)throw new BadRequestException('Unidade de outro condomínio.');}
  if(['people','vehicles','credentials'].includes(kind)&&!body.unit_id)throw new BadRequestException('Informe a unidade.');
  for(const [field,refKind] of Object.entries({block_id:'blocks',person_id:'people',device_id:'devices',access_id:'access-points'}))if(data[field]){const ref=await this.record(q,data[field],refKind);if(ref.condo_id!==body.condo_id||ref.unit_id&&ref.unit_id!==body.unit_id)throw new BadRequestException('Relacionamento fora do escopo.');}
  if(kind==='devices'&&data.homologated)throw new BadRequestException('Homologação exige o procedimento técnico documentado.');
  if(kind==='credentials')data.status='pending';
  const r=await one(q,'INSERT INTO records(company_id,condo_id,unit_id,kind,data) VALUES($1,$2,$3,$4,$5) RETURNING *',[s.company,body.condo_id,body.unit_id||null,kind,data]);
  await this.event(q,s,'record.created',r.id,r.condo_id,{kind},r.unit_id);return r;
 }
 async invite(q:Sql,s:Scope,input:any){
  requireRole(s,[...operators,'manager','resident']);await this.active(q,s);const b=invitationSchema.parse(input);
  const existing=await one(q,'SELECT id FROM invitations WHERE company_id=$1 AND request_key=$2',[s.company,b.request_key]);if(existing)throw new ConflictException({message:'Este convite já foi criado. Consulte a lista.',id:existing.id});
  const unit=await this.record(q,b.unit_id,'units');const from=new Date(b.valid_from),until=new Date(b.valid_until);
  if(until<=from||until<=new Date()||until.getTime()-from.getTime()>7*86400000)throw new BadRequestException('Use um período válido de até sete dias.');
  if(!(await one(q,'SELECT app.agent_online($1,$2) AS online',[s.company,unit.condo_id]))?.online)throw new ServiceUnavailableException('O condomínio está sem conexão.');
  for(const id of new Set(b.access_ids)){const a=await this.record(q,id,'access-points');if(a.condo_id!==unit.condo_id)throw new BadRequestException('Acesso de outro condomínio.');const device=await one(q,'SELECT app.device_ready($1,$2) AS ready',[s.company,a.data.device_id]);if(!device?.ready)throw new ConflictException('Equipamento ainda não homologado.');}
  const linkToken=token(),pin=String(randomInt(10000000,100000000));
  const row=await one(q,'INSERT INTO invitations(company_id,condo_id,unit_id,issuer,guest_name,vehicle_plate,valid_from,valid_until,access_ids,token_hash,credential_hash,credential_cipher,request_key,channel) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id,guest_name,valid_from,valid_until,status', [s.company,unit.condo_id,b.unit_id,s.subject,b.guest_name,b.vehicle_plate||null,from,until,[...new Set(b.access_ids)],hash(linkToken),hash(pin),seal(pin,this.key),b.request_key,b.channel]);
  await this.event(q,s,'invitation.created',row.id,unit.condo_id,{channel:b.channel},b.unit_id);
  return {...row,url:`${this.publicUrl}/convite/${linkToken}`};
 }
 async publicInvite(q:Sql,s:Scope,id:string){const i=await one(q,'SELECT * FROM invitations WHERE id=$1',[id]);if(!i)throw new NotFoundException();if(i.status!=='active'||new Date(i.valid_until)<new Date())throw new NotFoundException('Convite indisponível ou expirado.');const unit=await this.record(q,i.unit_id,'units');const c=await one(q,'SELECT name FROM condos WHERE id=$1',[i.condo_id]);return{guest_name:i.guest_name,valid_from:i.valid_from,valid_until:i.valid_until,condo:c?.name,unit:unit.data.name,credential:unseal(i.credential_cipher,this.key)};}
 async cancelInvite(q:Sql,s:Scope,id:string){requireRole(s,[...operators,'manager','resident']);const i=await one(q,"UPDATE invitations SET status='cancelled' WHERE id=$1 AND status='active' RETURNING *",[id]);if(!i)throw new ConflictException('Convite não está ativo.');await this.event(q,s,'invitation.cancelled',id,i.condo_id,{},i.unit_id);return{status:'cancelled'};}
 async open(q:Sql,s:Scope,input:any){requireRole(s,operators);await this.active(q,s);const b=openSchema.parse(input);const access=await this.record(q,b.access_id,'access-points');const device=await this.record(q,access.data.device_id,'devices');if(!device.data.homologated)throw new ConflictException('Equipamento não homologado.');if(!(await one(q,'SELECT app.agent_online($1,$2) AS online',[s.company,access.condo_id]))?.online)throw new ServiceUnavailableException('Agente desconectado.');const old=await one(q,'SELECT * FROM commands WHERE company_id=$1 AND request_key=$2',[s.company,b.request_key]);if(old)return old;
  if(access.data.mode==='both'){if(!b.authorization_id)throw new ForbiddenException('Este acesso exige uma credencial validada e conferência do operador.');const verified=await one(q,'UPDATE access_authorizations SET used=true WHERE id=$1 AND access_id=$2 AND NOT used AND expires_at>now() RETURNING id',[b.authorization_id,access.id]);if(!verified)throw new ForbiddenException('A autorização não está disponível ou expirou.');}
  const cmd=await one(q,"INSERT INTO commands(company_id,condo_id,access_id,device_id,actor,reason,request_key,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,now()+interval '10 seconds') RETURNING *",[s.company,access.condo_id,access.id,device.id,s.subject,b.reason,b.request_key]);await this.event(q,s,'access.authorized',cmd.id,access.condo_id,{reason:b.reason});return cmd;
 }
 async claim(q:Sql,s:Scope,id:string){requireRole(s,operators);const t=await one(q,"UPDATE tickets SET owner=$1,state='claimed' WHERE id=$2 AND state='waiting' RETURNING *",[s.subject,id]);if(!t)throw new ConflictException('Atendimento já assumido ou encerrado.');await this.event(q,s,'ticket.claimed',t.id,t.condo_id,{},t.unit_id);return t;}
 async close(q:Sql,s:Scope,id:string){requireRole(s,operators);const t=await one(q,"UPDATE tickets SET state='closed',closed_at=now() WHERE id=$1 AND state='claimed' AND owner=$2 RETURNING *",[id,s.subject]);if(!t)throw new ConflictException('Somente o responsável pode encerrar o atendimento.');await this.event(q,s,'ticket.closed',t.id,t.condo_id,{},t.unit_id);return t;}
 async dispatch(q:Sql,s:Scope){await q.query("UPDATE commands SET state='expired' WHERE state='pending' AND expires_at<=now()");await q.query("UPDATE commands SET state='unknown' WHERE state='sent' AND expires_at<=now()");const rows=(await q.query("UPDATE commands SET state='sent',dispatched_at=now() WHERE id IN (SELECT id FROM commands WHERE state='pending' AND expires_at>now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 10) RETURNING *")).rows;for(const r of rows)await this.event(q,s,'command.sent',r.id,r.condo_id);return rows;}
 async ack(q:Sql,s:Scope,id:string,state:'confirmed'|'failed'|'unknown'){const r=await one(q,"UPDATE commands SET state=$1,completed_at=now() WHERE id=$2 AND state='sent' AND expires_at>now() RETURNING *",[state,id]);if(!r)throw new ConflictException('Comando expirado ou já processado.');await this.event(q,s,`command.${state}`,id,r.condo_id);return r;}
 async redeem(q:Sql,s:Scope,credential:string,accessId:string,key:string){
  await this.active(q,s);const a=await this.record(q,accessId,'access-points');
  if(a.data.mode==='operator')throw new ForbiddenException('Este acesso exige conferência do operador.');
  if(!a.data.credential_types.some((x:string)=>['pin','qr'].includes(x)))throw new ForbiddenException('Credencial não aceita neste acesso.');
  const i=await one(q,"SELECT * FROM invitations WHERE credential_hash=$1 AND status='active' AND valid_from<=now() AND valid_until>now() AND $2=ANY(access_ids) FOR UPDATE",[hash(credential),accessId]);
  if(!i)throw new ForbiddenException('Credencial inválida, utilizada ou expirada.');const dev=await this.record(q,a.data.device_id,'devices');if(!dev.data.homologated)throw new ForbiddenException('Equipamento não homologado.');
  await q.query("UPDATE invitations SET status='used' WHERE id=$1",[i.id]);
  const approval=await one(q,"INSERT INTO access_authorizations(company_id,condo_id,invitation_id,access_id,used,expires_at) VALUES($1,$2,$3,$4,$5,least($6::timestamptz,now()+CASE WHEN $5 THEN interval '10 seconds' ELSE interval '2 minutes' END)) RETURNING id,expires_at",[s.company,i.condo_id,i.id,accessId,a.data.mode==='automatic',i.valid_until]);
  await this.event(q,s,'credential.authorized',i.id,i.condo_id,{access_id:accessId,authorization_id:approval.id,requires_operator:a.data.mode==='both'},i.unit_id);
  if(a.data.mode==='both')await q.query('INSERT INTO tickets(company_id,condo_id,unit_id,access_id,title) VALUES($1,$2,$3,$4,$5)',[s.company,i.condo_id,i.unit_id,accessId,`Conferir convidado: ${i.guest_name}`]);
  let commandId:string|undefined;
  if(a.data.mode==='automatic'){const command=await one(q,"INSERT INTO commands(company_id,condo_id,access_id,device_id,actor,reason,request_key,state,dispatched_at,expires_at) VALUES($1,$2,$3,$4,$5,'Credencial temporária validada',$6,'sent',now(),$7) RETURNING id",[s.company,i.condo_id,accessId,dev.id,s.subject,`credential-${approval.id}`,approval.expires_at]);commandId=command.id;await this.event(q,s,'command.sent',commandId,i.condo_id,{authorization_id:approval.id},i.unit_id);}
  return{authorized:a.data.mode==='automatic',requires_operator:a.data.mode==='both',authorization_id:approval.id,command_id:commandId,device_id:dev.id,request_key:key,expires_at:approval.expires_at};
 }
}
