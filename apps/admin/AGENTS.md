# 管理后台

React + TypeScript、React Router、Axios、Ant Design。继承根规则。页面按业务模块组织，API 层集中处理身份、响应、错误、幂等键。页面覆盖加载、空数据、错误、提交中、无权限和并发冲突。

输入金额按元字符串转换成分字符串；不使用 Number/parseFloat 计算金额。权限由后端强制执行。敏感操作显示影响及确认，不显示实现细节。测试使用 Vitest + Testing Library，端到端按实际闭环验证。
