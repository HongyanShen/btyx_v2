const weights = [7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2];

function identityAgeError(value, now = new Date()) {
  const number = value.trim().toUpperCase();
  const invalid = "请输入有效的 18 位身份证号";
  if (!/^[0-9]{17}[0-9X]$/.test(number)) return invalid;

  const checksum = weights.reduce((sum, weight, index) => {
    return sum + Number(number[index]) * weight;
  }, 0);
  if (number[17] !== "10X98765432"[checksum % 11]) return invalid;

  const year = Number(number.slice(6, 10));
  const month = Number(number.slice(10, 12));
  const day = Number(number.slice(12, 14));
  if (year < 1800) return invalid;

  const birthday = new Date(Date.UTC(year, month - 1, day));
  const beijingDate = new Date(now.getTime() + 8 * 60 * 60 * 1000);
  if (
    birthday.getUTCMonth() !== month - 1 ||
    birthday.getUTCDate() !== day ||
    birthday > beijingDate
  ) {
    return invalid;
  }

  if (new Date(Date.UTC(year + 18, month - 1, day)) > beijingDate) {
    return "实名认证须年满 18 周岁";
  }

  return "";
}

const identityPhotoEvents = {
  photoBusy(event) {
    this.setData({ uploading: event.detail.busy });
  },

  photoChanged(event) {
    const { side, fileID } = event.detail;
    const fields = { [`document.${side}_file_id`]: fileID };
    if (side === "front") {
      fields["document.name"] = "";
      fields["document.identity"] = "";
      fields.ageError = "";
    }

    this.setData({ ...fields, error: "" });
  },

  identityRecognized(event) {
    const { name, identity } = event.detail;
    this.setData({
      "document.name": name,
      "document.identity": identity,
      ageError: identityAgeError(identity),
      error: "",
    });
  },
};

module.exports = { identityAgeError, identityPhotoEvents };
