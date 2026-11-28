const pages = {
  orders: { title: "订单明细", icon: "/assets/me/order.png" },
  points: { title: "点数", icon: "/assets/me/points.png" },
};

Page({
  data: { page: pages.orders },

  onLoad({ type }) {
    const page = type === "points" ? pages.points : pages.orders;
    this.setData({ page });
    wx.setNavigationBarTitle({ title: page.title });
  },
});
