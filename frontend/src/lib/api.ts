const API_BASE = '';

export async function apiGet<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, { headers: { 'cache-control': 'no-store' } });
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json() as Promise<T>;
}

export type NetInterface = {
  name: string;
  rxBytes: number;
  txBytes: number;
  rxBps: number;
  txBps: number;
};

export type NetPort = {
  proto: string;
  localAddr: string;
  localPort: number;
  state: string;
  connections: number;
};
