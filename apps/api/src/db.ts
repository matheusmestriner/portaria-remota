import { Pool } from 'pg';
export type Sql = { query: (sql: string, params?: any[]) => Promise<{ rows: any[]; rowCount?: number | null }> };
export type Scope = { subject: string; platform?: boolean; company?: string; role?: string; condos?: string[]; units?: string[] };
export class Database {
  constructor(public pool: Pool) {}
  async transaction<T>(scope: Scope, fn: (q: Sql) => Promise<T>): Promise<T> {
    const q = await this.pool.connect();
    try { await q.query('BEGIN'); await applyScope(q,scope); const result=await fn(q); await q.query('COMMIT'); return result; }
    catch(e) { await q.query('ROLLBACK'); throw e; } finally { q.release(); }
  }
}
export async function applyScope(q: Sql, s: Scope) {
  await q.query('SET LOCAL ROLE portaria_app');
  for(const [key,value] of Object.entries({subject:s.subject, platform:String(!!s.platform), company:s.company||'',role:s.role||'',condos:(s.condos||[]).join(','),units:(s.units||[]).join(',')}))
    await q.query('SELECT set_config($1,$2,true)',[`app.${key}`,value]);
}
export async function one(q:Sql,sql:string,args:any[]=[]){return (await q.query(sql,args)).rows[0];}
