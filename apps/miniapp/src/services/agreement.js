const { request } = require("./request");

async function getCurrentAgreement() {
  try {
    return await request("/agreement");
  } catch (error) {
    // This endpoint returns 409 when no agreement has been published.
    if (error.status === 409) return null;

    throw error;
  }
}

module.exports = { getCurrentAgreement };
