const watermark = "仅供白兔严选实名认证使用";
const maxImageSide = 1600;

async function addIdentityWatermark(canvas, sourcePath) {
  if (!canvas) throw new Error("无法为照片添加水印，请重试");

  const image = await new Promise((resolve, reject) => {
    const photo = canvas.createImage();
    photo.onload = () => resolve(photo);
    photo.onerror = () => reject(new Error("无法读取身份证照片"));
    photo.src = sourcePath;
  });

  if (!image.width || !image.height) throw new Error("身份证照片尺寸无效");

  const scale = Math.min(1, maxImageSide / Math.max(image.width, image.height));
  const width = Math.max(1, Math.round(image.width * scale));
  const height = Math.max(1, Math.round(image.height * scale));
  canvas.width = width;
  canvas.height = height;

  const context = canvas.getContext("2d");
  context.fillStyle = "#fff";
  context.fillRect(0, 0, width, height);
  context.drawImage(image, 0, 0, width, height);

  const fontSize = Math.round(Math.min(width, height) * 0.043);
  context.font = `600 ${Math.max(10, fontSize)}px sans-serif`;
  context.textAlign = "center";
  context.textBaseline = "middle";
  for (const position of [0.25, 0.5, 0.75]) {
    context.save();
    context.translate(width / 2, height * position);
    context.rotate(-Math.PI / 8);
    context.fillStyle = "rgba(255, 255, 255, 0.42)";
    context.fillText(watermark, 1, 1, width * 0.9);
    context.fillStyle = "rgba(48, 36, 75, 0.31)";
    context.fillText(watermark, 0, 0, width * 0.9);
    context.restore();
  }

  return new Promise((resolve, reject) => {
    wx.canvasToTempFilePath({
      canvas,
      fileType: "jpg",
      quality: 0.94,
      destWidth: width,
      destHeight: height,
      success: ({ tempFilePath }) => resolve(tempFilePath),
      fail: () => reject(new Error("无法保存加水印后的照片，请重试")),
    });
  });
}

module.exports = { addIdentityWatermark };
