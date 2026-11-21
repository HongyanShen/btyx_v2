function searchKeyword(value: string | undefined, platformHosts: string[]) {
  const account = value?.trim().replace(/^@/, "");
  if (!account) return;

  const address = /^(?:[\w-]+\.)+[a-z]+\//i.test(account)
    ? `https://${account}`
    : account;
  const candidate = address.match(
    /https?:\/\/[^\s<>"'，。！？、）】》]+/iu,
  )?.[0];
  if (!candidate) return /[:/<>]/.test(account) ? undefined : account;

  try {
    const url = new URL(candidate);
    if (url.username || url.password || !platformHosts.includes(url.host))
      return;

    // Share parameters belong to the original link and are unnecessary for search.
    return `https://${url.host}${url.pathname}`;
  } catch {
    return undefined;
  }
}

export function douyinSearchURL(value?: string) {
  const keyword = searchKeyword(value, [
    "douyin.com",
    "www.douyin.com",
    "v.douyin.com",
  ]);
  if (keyword) {
    return `https://www.douyin.com/search/${encodeURIComponent(keyword)}?type=user`;
  }
}

export function xiaohongshuSearchURL(value?: string) {
  const keyword = searchKeyword(value, [
    "xiaohongshu.com",
    "www.xiaohongshu.com",
    "xhslink.com",
  ]);
  if (keyword) {
    return `https://www.xiaohongshu.com/search_result?keyword=${encodeURIComponent(keyword)}`;
  }
}
