export function GET(req:Request){return Response.json({service:'portaria',host:req.headers.get('host')?.split(':')[0]},{headers:{'Cache-Control':'no-store'}});}
