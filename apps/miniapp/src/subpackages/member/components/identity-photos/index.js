const { chooseImage, upload } = require("../../../../services/files");
const {
  addIdentityWatermark,
} = require("../../../../services/identity-watermark");
const { post } = require("../../../../services/request");

Component({
  properties: {
    disabled: Boolean,
    savedFront: String,
    savedBack: String,
    retainedFront: Boolean,
    retainedBack: Boolean,
  },

  data: {
    front: "",
    back: "",
    busy: "",
    recognizing: false,
    uploadError: "",
    recognitionError: "",
  },

  methods: {
    setBusy(side) {
      this.setData({ busy: side });
      this.triggerEvent("busy", { busy: !!side });
    },

    async choosePhoto(event) {
      if (this.data.busy || this.data.disabled) return;

      const { side } = event.currentTarget.dataset;
      this.setBusy(side);

      try {
        let source;
        try {
          source = await chooseImage();
        } catch {
          return;
        }

        this.setData({ uploadError: "" });
        const canvas = await new Promise((resolve) => {
          this.createSelectorQuery()
            .select(".identity-canvas")
            .fields({ node: true }, (result) => resolve(result?.node))
            .exec();
        });
        const watermarked = await addIdentityWatermark(canvas, source);
        const fileID = await upload(watermarked);
        this.setData({ [side]: watermarked });
        this.triggerEvent("photo", { side, fileID });

        if (side === "front") {
          this.frontFileID = fileID;
          await this.recognize();
        }
      } catch (error) {
        this.setData({ uploadError: error.message });
      } finally {
        this.setBusy("");
      }
    },

    async retryRecognition() {
      if (this.data.busy || this.data.disabled || !this.frontFileID) return;

      this.setBusy("front");
      try {
        await this.recognize();
      } finally {
        this.setBusy("");
      }
    },

    async recognize() {
      this.setData({ recognizing: true, recognitionError: "" });
      try {
        const result = await post("/me/identity/recognize", {
          file_id: this.frontFileID,
        });
        this.triggerEvent("recognize", result);
      } catch (error) {
        this.setData({ recognitionError: error.message });
      } finally {
        this.setData({ recognizing: false });
      }
    },
  },
});
