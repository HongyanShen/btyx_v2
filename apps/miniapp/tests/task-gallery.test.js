const assert = require("node:assert/strict");
const test = require("node:test");

test("多图预览按顺序打开，缩略图和滑动共用选中项，下载当前图片", async () => {
  const prior = { Page: global.Page, wx: global.wx };
  const signedRequests = [];
  const downloads = [];
  const album = [];
  let page;

  global.Page = (definition) => {
    page = {
      ...definition,
      setData(data) {
        Object.assign(this.data, data);
      },
    };
  };
  global.wx = {
    getStorageSync: () => "session",
    showToast() {},
    request(options) {
      const path = new URL(options.url).pathname;
      signedRequests.push(path);
      const name = path.split("/").at(-2);
      options.success({
        statusCode: 200,
        data: {
          code: 0,
          data: {
            name: `${name}.jpg`,
            media_type: "image/jpeg",
            storage: "tos",
            url: `https://example.com/${name}`,
          },
        },
      });
    },
    downloadFile(options) {
      downloads.push(options);
      options.success({ statusCode: 200, tempFilePath: "local-image" });
    },
    saveImageToPhotosAlbum(options) {
      album.push(options.filePath);
      options.success();
    },
  };

  try {
    require("../src/subpackages/task/detail/index");
    page.openGallery();
    assert.equal(page.data.galleryOpen, false);

    await page.loadMaterials(["one", "two", "three"]);
    assert.deepEqual(
      page.data.materials.images.map(({ id }) => id),
      ["one", "two", "three"],
    );
    page.openGallery();
    assert.equal(page.data.galleryOpen, true);
    assert.equal(page.data.galleryIndex, 0);

    page.selectGalleryImage({ currentTarget: { dataset: { index: 2 } } });
    assert.equal(page.data.galleryIndex, 2);
    page.selectGalleryImage({ detail: { current: 0, source: "" } });
    assert.equal(page.data.galleryIndex, 2);
    page.selectGalleryImage({ detail: { current: 1, source: "touch" } });
    assert.equal(page.data.galleryIndex, 1);
    page.selectGalleryImage({ detail: { current: 8 } });
    assert.equal(page.data.galleryIndex, 1);

    await page.saveGalleryImage();
    assert.equal(signedRequests.at(-1), "/api/v1/files/two/download");
    assert.equal(downloads[0].url, "https://example.com/two");
    assert.equal(downloads[0].header, undefined);
    assert.deepEqual(album, ["local-image"]);
    assert.equal(page.data.savedFiles.two, true);

    page.closeGallery();
    assert.equal(page.data.galleryOpen, false);
    page.data.task = { files: ["one", "two", "three"] };
    page.data.materials.images[0].expiresAt = "2000-01-01T00:00:00Z";
    const requestCount = signedRequests.length;
    await page.openGallery();
    assert.equal(page.data.galleryIndex, 0);
    assert.equal(signedRequests.length, requestCount + 3);
    assert.deepEqual(
      page.data.materials.images.map(({ id }) => id),
      ["one", "two", "three"],
    );
  } finally {
    global.Page = prior.Page;
    global.wx = prior.wx;
  }
});
