const { request } = require("./request");

async function profilePhotoURL(fileID) {
  if (!fileID) return "";

  try {
    const ticket = await request(
      `/files/${encodeURIComponent(fileID)}/download`,
    );
    return ticket.url || "";
  } catch {
    return "";
  }
}

module.exports = { profilePhotoURL };
