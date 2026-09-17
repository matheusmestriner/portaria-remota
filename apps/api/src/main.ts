import 'reflect-metadata';
import { NestFactory } from '@nestjs/core';
import { Module,Catch,type ExceptionFilter,type ArgumentsHost,HttpException,ValidationPipe } from '@nestjs/common';
import { SwaggerModule,DocumentBuilder } from '@nestjs/swagger';
import { ZodError } from 'zod';
import helmet from 'helmet';
import { ApiController } from './controller';
import { IntegrationController } from './integrations';
import { env,db } from './runtime';
@Catch() class Errors implements ExceptionFilter {catch(error:any,host:ArgumentsHost){const res=host.switchToHttp().getResponse();if(error instanceof ZodError)return res.status(400).json({message:'Revise os campos informados.',details:error.issues.map(i=>({field:i.path.join('.'),message:i.message}))});if(error instanceof HttpException)return res.status(error.getStatus()).json(typeof error.getResponse()==='string'?{message:error.message}:error.getResponse());if(['23505','23503','23514','42501','22P02'].includes(error.code))return res.status(error.code==='23505'?409:400).json({message:error.code==='23505'?'Este cadastro já existe.':'Operação não permitida. Verifique os vínculos e os campos.'});console.error('request_failed',{name:error?.name,code:error?.code});return res.status(500).json({message:'Não foi possível concluir a operação.'});}}
@Module({controllers:[ApiController,IntegrationController]}) class AppModule {}
async function main(){
 if(!env.DATABASE_URL||!env.DATA_KEY||!env.OIDC_ISSUER)throw Error('Configure DATABASE_URL, DATA_KEY e OIDC_ISSUER no .env. Consulte README.md.');
 const row=(await db.pool.query('SELECT rolsuper,rolbypassrls FROM pg_roles WHERE rolname=current_user')).rows[0];if(row?.rolsuper||row?.rolbypassrls)throw Error('A API exige uma conexão PostgreSQL sem privilégios de superusuário/BYPASSRLS.');
 if(!/^[a-f0-9]{64}$/i.test(env.DATA_KEY))throw Error('DATA_KEY precisa conter 32 bytes em hexadecimal.');
 const app=await NestFactory.create(AppModule,{bodyParser:true});if(env.TRUST_PROXY==='1')app.getHttpAdapter().getInstance().set('trust proxy',1);app.use(helmet());app.useGlobalFilters(new Errors());
 app.enableCors({origin:(env.WEB_ORIGINS||'http://localhost:3000,http://127.0.0.1:3000').split(','),allowedHeaders:['Authorization','Content-Type','X-Company-Id'],methods:['GET','POST','PATCH','OPTIONS']});
 const rate=new Map<string,{start:number,n:number}>();app.use((req:any,res:any,next:any)=>{const key=req.ip;const now=Date.now();if(rate.size>10000)for(const [k,v]of rate)if(now-v.start>60000)rate.delete(k);const item=rate.get(key);if(!item||now-item.start>60000){rate.set(key,{start:now,n:1});return next();}if(++item.n>240)return res.status(429).json({message:'Muitas solicitações. Aguarde um minuto.'});next();});
 const document=SwaggerModule.createDocument(app,new DocumentBuilder().setTitle('Portaria API').setDescription('API v1. Operações de empresa exigem X-Company-Id. Esquemas de entrada são validados no servidor.').setVersion('1.0').addBearerAuth().build());SwaggerModule.setup('docs',app,document);
 app.enableShutdownHooks();await app.listen(Number(process.env.PORT||env.API_PORT||4000),env.API_HOST||'0.0.0.0');
}
main().catch(e=>{console.error(e.message);process.exitCode=1;});
