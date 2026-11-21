const { FileMediaType } = require("./generated/enums");

function mediaType(bytes) {
  const text = (start, count) =>
    String.fromCharCode(...bytes.slice(start, start + count));
  if (bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff) {
    return FileMediaType.FileJPEG;
  }

  if (
    [137, 80, 78, 71, 13, 10, 26, 10].every(
      (value, index) => bytes[index] === value,
    )
  ) {
    return FileMediaType.FilePNG;
  }

  if (text(0, 6) === "GIF87a" || text(0, 6) === "GIF89a") {
    return FileMediaType.FileGIF;
  }

  if (text(0, 4) === "RIFF" && text(8, 4) === "WEBP") {
    return FileMediaType.FileWEBP;
  }

  if (text(4, 4) === "ftyp") {
    return FileMediaType.FileMP4;
  }

  throw new Error("仅支持 JPG、PNG、GIF、WEBP 图片及 MP4 视频");
}

async function fileInfo(filePath) {
  const fs = wx.getFileSystemManager();
  const size = await new Promise((resolve, reject) => {
    fs.stat({
      path: filePath,
      success: (result) => resolve(result.stats.size),
      fail: () => reject(new Error("无法读取文件信息")),
    });
  });
  if (!Number.isSafeInteger(size) || size < 1 || size > 25 * 1024 * 1024) {
    throw new Error("请选择25MB以内的非空文件");
  }

  const prefix = await new Promise((resolve, reject) => {
    fs.readFile({
      filePath,
      position: 0,
      length: Math.min(size, 512),
      success: (result) => resolve(new Uint8Array(result.data)),
      fail: () => reject(new Error("无法读取文件内容")),
    });
  });

  return {
    name: filePath.split("/").pop(),
    media_type: mediaType(prefix),
    size,
  };
}

module.exports = { fileInfo };
