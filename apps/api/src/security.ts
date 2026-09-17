import { createHash,randomBytes,createCipheriv,createDecipheriv } from 'node:crypto';
import { createRemoteJWKSet,jwtVerify } from 'jose';
import { ForbiddenException,UnauthorizedException } from '@nestjs/common';
import type { Database,Scope } from './db';
export const hash=(value:string)=>createHash('sha256').update(value).digest('hex');
export const token=()=>randomBytes(32).toString('base64url');
export function seal(value:string,key:string){const k=Buffer.from(key,'hex'); if(k.length!==32)throw Error('DATA_KEY must contain 32 bytes');const iv=randomBytes(12);const c=createCipheriv('aes-256-gcm',k,iv);const encrypted=Buffer.concat([c.update(value,'utf8'),c.final()]);return [iv,c.getAuthTag(),encrypted].map(b=>b.toString('base64url')).join('.');}
export function unseal(value:string,key:string){const [iv,tag,data]=value.split('.').map(s=>Buffer.from(s,'base64url'));const c=createDecipheriv('aes-256-gcm',Buffer.from(key,'hex'),iv);c.setAuthTag(tag);return Buffer.concat([c.update(data),c.final()]).toString('utf8');}
export type Identity={subject:string;platform:boolean;mfa:boolean;memberships:any[]};
export class Auth {
  private jwks; constructor(private db:Database,private issuer:string,private audience:string,jwksUrl?:string){this.jwks=createRemoteJWKSet(new URL(jwksUrl||`${issuer}/protocol/openid-connect/certs`));}
  async identity(header?:string):Promise<Identity>{
    if(!header?.startsWith('Bearer '))throw new UnauthorizedException('Entre na sua conta para continuar.');
    let payload;try{payload=(await jwtVerify(header.slice(7),this.jwks,{issuer:this.issuer,audience:this.audience,algorithms:['RS256']})).payload;}catch{throw new UnauthorizedException('Sessão inválida ou expirada.');}
    if(!payload.sub)throw new UnauthorizedException();
    const mfa=Number(payload.acr)>=2;
    return this.db.transaction({subject:payload.sub},async q=>{
      const platform=(await q.query('SELECT subject FROM platform_admins WHERE subject=$1',[payload.sub])).rows.length>0;
      const memberships=(await q.query('SELECT *,app.company_label(company_id) AS company_name FROM memberships WHERE subject=$1 AND active',[payload.sub])).rows;
      if((platform||memberships.some(m=>['company_admin','supervisor','operator'].includes(m.role)))&&!mfa)throw new ForbiddenException('Esta função exige autenticação em dois fatores.');
      return {subject:payload.sub!,platform,mfa,memberships};
    });
  }
  async scope(id:Identity,company?:string):Promise<Scope>{
    if(!company) {if(!id.platform)throw new ForbiddenException('Selecione uma empresa autorizada.');return{subject:id.subject,platform:true};}
    const member=id.memberships.find(m=>m.company_id===company);
    if(member) return{subject:id.subject,platform:id.platform,company,role:member.role,condos:member.condo_ids,units:member.unit_ids};
    if(id.platform){const allowed=await this.db.transaction({subject:id.subject,platform:true},async q=>(await q.query('SELECT id FROM support_grants WHERE company_id=$1 AND subject=$2 AND expires_at>now()',[company,id.subject])).rows.length>0);if(allowed)return{subject:id.subject,platform:true,company,role:'support'};}
    throw new ForbiddenException('Você não tem permissão nesta empresa. Inicie uma sessão de suporte auditada, se aplicável.');
  }
}
export function requireRole(s:Scope,roles:string[]){if(!s.role||!roles.includes(s.role))throw new ForbiddenException('Seu perfil não permite esta ação.');}
