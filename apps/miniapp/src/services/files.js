const config = require("./config");
const { tokenKey, request } = require("./request");
const { FileMediaType, FileStorage } = require("./generated/enums");
const { upload } = require("./file-upload");

function chooseImage() {
  return new Promise((resolve, reject) => {
    const cancel = () => reject(new Error("未选择图片"));
    const selected = (path) => (path ? resolve(path) : cancel());
    if (typeof wx.chooseMedia === "function") {
      wx.chooseMedia({
        count: 1,
        mediaType: ["image"],
        success: (result) => selected(result.tempFiles?.[0]?.tempFilePath),
        fail: cancel,
      });
      return;
    }

    wx.chooseImage({
      count: 1,
      success: (result) => selected(result.tempFilePaths?.[0]),
      fail: cancel,
    });
  });
}

function downloadMaterial(id, ticket) {
  const legacy = ticket.storage === FileStorage.FileStorageLocal;
  return new Promise((resolve, reject) => {
    wx.downloadFile({
      url: legacy ? `${config.apiBase}/files/${id}` : ticket.url,
      ...(legacy
        ? { header: { Authorization: `Bearer ${wx.getStorageSync(tokenKey)}` } }
        : {}),
      success(response) {
        if (response.statusCode !== 200) {
          reject(new Error("素材下载失败"));

          return;
        }

        resolve(response.tempFilePath);
      },
      fail() {
        reject(new Error("素材下载失败"));
      },
    });
  });
}

async function materialTicket(id) {
  const ticket = await request(`/files/${id}/download`);
  if (
    ticket.storage === FileStorage.FileStorageLocal &&
    ticket.media_type?.startsWith("image/")
  ) {
    const previewPath = await downloadMaterial(id, ticket).catch(() => "");
    return { ...ticket, previewPath };
  }

  return ticket;
}

async function saveMaterial(id) {
  const ticket = await request(`/files/${id}/download`);
  const filePath = await downloadMaterial(id, ticket);
  return new Promise((resolve, reject) => {
    const options = {
      filePath,
      success: resolve,
      fail: () => reject(new Error("保存失败，请检查相册权限与文件类型")),
    };
    if (ticket.media_type === FileMediaType.FileMP4) {
      wx.saveVideoToPhotosAlbum(options);
    } else {
      wx.saveImageToPhotosAlbum(options);
    }
  });
}

module.exports = { chooseImage, materialTicket, upload, saveMaterial };
