import { z } from 'zod';
export const uuid=z.uuid();
const text=z.string().trim().min(1).max(200);
export const companySchema=z.object({name:text,slug:z.string().regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/).max(60)}).strict();
export const brandSchema=z.object({name:text,primary:z.string().regex(/^#[0-9a-fA-F]{6}$/),logo:z.url().refine(v=>v.startsWith('https://')).optional()}).strict();
export const membershipSchema=z.object({subject:text,name:text,role:z.enum(['company_admin','supervisor','operator','manager','resident']),condo_ids:z.array(uuid).default([]),unit_ids:z.array(uuid).default([])}).strict();
export const invitationSchema=z.object({unit_id:uuid,guest_name:text,vehicle_plate:z.string().regex(/^[A-Z0-9-]{5,10}$/).optional(),valid_from:z.iso.datetime({offset:true}),valid_until:z.iso.datetime({offset:true}),access_ids:z.array(uuid).min(1).max(20),request_key:z.string().min(16).max(100),channel:z.enum(['web','app','whatsapp']).default('web')}).strict();
export const recordSchemas:Record<string,z.ZodType>={
 'blocks':z.object({name:text}),
 'units':z.object({name:text,block_id:uuid.optional()}),
 'people':z.object({name:text,type:z.enum(['resident','visitor','provider']),phone:z.string().regex(/^\+[1-9]\d{7,14}$/).optional()}),
 'vehicles':z.object({name:text,plate:z.string().trim().toUpperCase().regex(/^[A-Z0-9-]{5,10}$/),person_id:uuid}),
 'access-points':z.object({name:text,mode:z.enum(['automatic','operator','both']),device_id:uuid,credential_types:z.array(z.enum(['qr','pin','tag','card','face','plate'])).min(1)}),
 'devices':z.object({name:text,adapter:z.literal('http-relay-v1'),address:z.string().max(200),capabilities:z.array(z.enum(['qr','pin','tag','card','face','plate'])).default([]),homologated:z.boolean().default(false)}),
 'credentials':z.object({name:text,person_id:uuid,type:z.enum(['tag','card','face','plate']),reference:text,device_id:uuid,status:z.enum(['pending','active','revocation_pending','revoked']).default('pending')}),
 'cameras':z.object({name:text,access_id:uuid,stream_path:z.string().regex(/^[a-zA-Z0-9_/-]+$/)}),
 'intercoms':z.object({name:text,access_id:uuid,extension:z.string().regex(/^\d{2,12}$/)}),
 'rules':z.object({name:text,access_id:uuid,allow_recurring:z.boolean().default(false)}),
 'incidents':z.object({name:text,description:z.string().min(1).max(4000),severity:z.enum(['info','warning','critical']),status:z.enum(['open','closed']).default('open')})
};
export const openSchema=z.object({access_id:uuid,reason:z.string().trim().min(5).max(500),request_key:z.string().min(16).max(100),authorization_id:uuid.optional()}).strict();
