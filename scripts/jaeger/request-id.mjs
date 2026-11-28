import { createServer } from "node:http";
import { readFile } from "node:fs/promises";

const page = await readFile(new URL("./request-id.html", import.meta.url), "utf8");
const jaeger = "http://127.0.0.1:16686";
const retention = 7 * 24 * 60 * 60 * 1000;

async function findTrace(service, requestID, now) {
  const url = new URL("/api/v3/trace-summaries", jaeger);
  url.search = new URLSearchParams({
    "query.serviceName": service,
    "query.attributes": JSON.stringify({ request_id: requestID }),
    "query.startTimeMin": new Date(now - retention).toISOString(),
    "query.startTimeMax": new Date(now).toISOString(),
    "query.searchDepth": "1",
  });
  const response = await fetch(url, { signal: AbortSignal.timeout(5000) });
  if (!response.ok) throw new Error("Jaeger query failed");
  const { summaries } = await response.json();
  return summaries?.[0]?.traceId;
}

createServer(async (req, res) => {
  const url = new URL(req.url, "http://127.0.0.1:16687");
  if (req.method !== "GET" || url.pathname !== "/") {
    res.writeHead(404).end();
    return;
  }
  res.setHeader("Cache-Control", "no-store");
  res.setHeader("Referrer-Policy", "no-referrer");
  const requestID = url.searchParams.get("id")?.trim();
  let status = 200;
  let message = "";
  if (url.searchParams.has("id")) {
    if (!/^[0-9A-Za-z]{27}$/.test(requestID)) {
      status = 400;
      message = "请输入完整的 27 位 X-Request-ID。";
    } else {
      try {
        const now = Date.now();
        const matches = await Promise.all(
          ["admin", "miniapi", "worker"].map((service) => findTrace(service, requestID, now)),
        );
        const id = matches.find((value) => /^[0-9a-f]{32}$/i.test(value));
        if (id) {
          res.writeHead(303, { Location: jaeger + "/trace/" + id }).end();
          return;
        }
        status = 404;
        message = "最近 7 天未查到这条链路。如果请求刚结束，请稍等几秒再查询。";
      } catch {
        status = 503;
        message = "暂时无法查询本机 Jaeger，请稍后重试。";
      }
    }
  }
  res.writeHead(status, { "Content-Type": "text/html; charset=utf-8" });
  res.end(page.replace("<!--message-->", message).replace("<!--id-->", /^[0-9A-Za-z]{27}$/.test(requestID) ? requestID : ""));
}).listen(16687, "127.0.0.1");
