import { settings } from '../../server-settings';
export function GET(){return Response.json({configured:!!settings.issuer,issuer:settings.issuer||'',clientId:'portaria-web',api:'/api/backend',domainTarget:settings.domainTarget||''});}
