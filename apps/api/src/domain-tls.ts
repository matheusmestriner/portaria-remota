import { resolve4,resolveCname } from 'node:dns/promises';
import { request } from 'node:https';
export function publicIPv4(ip:string){const p=ip.split('.').map(Number);if(p.length!==4||p.some(n=>!Number.isInteger(n)||n<0||n>255))return false;return !(p[0]===0||p[0]===10||p[0]===127||p[0]>=224||p[0]===169&&p[1]===254||p[0]===172&&p[1]>=16&&p[1]<=31||p[0]===192&&p[1]===168||p[0]===100&&p[1]>=64&&p[1]<=127||p[0]===198&&(p[1]===18||p[1]===19));}
export async function verifyDomainTLS(host:string,target:string){
 const cnames=await resolveCname(host);if(!cnames.map(s=>s.replace(/\.$/,'')).includes(target))return false;
 const addresses=await resolve4(host);if(!addresses.length||addresses.some(ip=>!publicIPv4(ip)))return false;
 return new Promise<boolean>(resolve=>{const req=request({hostname:host,servername:host,path:'/api/domain-check',method:'GET',port:443,timeout:7000,rejectUnauthorized:true,lookup:((_host:any,_opts:any,callback:any)=>callback(null,addresses[0],4)) as any},res=>{let data='';res.on('data',chunk=>{data+=chunk;if(data.length>2048){req.destroy();resolve(false);}});res.on('end',()=>{try{const b=JSON.parse(data);resolve(res.statusCode===200&&b.host===host&&b.service==='portaria');}catch{resolve(false);}});});req.on('timeout',()=>{req.destroy();resolve(false);});req.on('error',()=>resolve(false));req.end();});
}
