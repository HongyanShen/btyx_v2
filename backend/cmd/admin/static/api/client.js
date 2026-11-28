import axios from "../vendor/axios.js";
const client = axios.create({
  baseURL: "/api/v1",
  timeout: 2e4,
});
client.interceptors.request.use((config) => {
  const workspace = document.querySelector("#admin-workspace");
  const csrf = workspace?.dataset.csrf;
  if (csrf && config.method !== "get" && config.method !== "head") {
    config.headers.set("X-CSRF-Token", csrf);
  }
  return config;
});
client.interceptors.response.use(undefined, (error) => {
  if (axios.isAxiosError(error)) {
    if (error.response?.status === 401) {
      window.dispatchEvent(new Event("auth-expired"));
    } else if (
      error.response?.status === 403 &&
      error.config?.url !== "/auth/me"
    ) {
      window.dispatchEvent(new Event("access-forbidden"));
    }
    throw new Error(error.response?.data?.msg ?? "请求失败，请稍后重试");
  }
  throw error;
});
export async function get(path) {
  const response = await client.get(path);
  return response.data.data;
}
export async function getPage(path) {
  const response = await client.get(path);
  const count = response.headers["x-total-count"];
  const total = Number(count);
  if (
    typeof count !== "string" ||
    !/^\d+$/.test(count) ||
    !Number.isSafeInteger(total)
  ) {
    throw new Error("未收到有效的分页信息，请刷新后重试");
  }
  return {
    rows: response.data.data,
    total,
  };
}
export async function post(path, body, idempotencyKey) {
  const response = await client.post(path, body, {
    headers: idempotencyKey ? { "Idempotency-Key": idempotencyKey } : undefined,
  });
  return response.data.data;
}
export async function getBlob(path) {
  const response = await client.get(path, { responseType: "blob" });
  return response.data;
}
