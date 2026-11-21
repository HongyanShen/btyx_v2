import axios from "axios";

interface Envelope<T> {
  code: number;
  msg: string;
  data: T;
}

export const sessionKey = "btyx-admin-session";

const client = axios.create({
  baseURL: import.meta.env.VITE_API_BASE ?? "/api/v1",
  timeout: 20_000,
});

client.interceptors.request.use((config) => {
  const token = sessionStorage.getItem(sessionKey);
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }

  return config;
});

client.interceptors.response.use(undefined, (error: unknown) => {
  if (axios.isAxiosError<Envelope<null>>(error)) {
    const token = sessionStorage.getItem(sessionKey);
    if (token && error.config?.headers.Authorization === `Bearer ${token}`) {
      if (error.response?.status === 401) {
        sessionStorage.removeItem(sessionKey);
        window.dispatchEvent(new Event("auth-expired"));
      } else if (
        error.response?.status === 403 &&
        error.config?.url !== "/auth/me"
      ) {
        window.dispatchEvent(new Event("access-forbidden"));
      }
    }

    throw new Error(error.response?.data?.msg ?? "请求失败，请稍后重试");
  }

  throw error;
});

export async function get<T>(path: string): Promise<T> {
  const response = await client.get<Envelope<T>>(path);

  return response.data.data;
}

export async function getPage<T>(
  path: string,
): Promise<{ rows: T[]; total: number }> {
  const response = await client.get<Envelope<T[]>>(path);
  const count = response.headers["x-total-count"];
  const total = Number(count);
  if (
    typeof count !== "string" ||
    !/^\d+$/.test(count) ||
    !Number.isSafeInteger(total)
  ) {
    throw new Error("未收到有效的分页信息，请刷新后重试");
  }

  return { rows: response.data.data, total };
}

export async function post<T = null>(
  path: string,
  body: unknown,
  idempotencyKey?: string,
): Promise<T> {
  const response = await client.post<Envelope<T>>(path, body, {
    headers: idempotencyKey ? { "Idempotency-Key": idempotencyKey } : undefined,
  });

  return response.data.data;
}

export async function getBlob(path: string): Promise<Blob> {
  const response = await client.get<Blob>(path, {
    responseType: "blob",
  });

  return response.data;
}
