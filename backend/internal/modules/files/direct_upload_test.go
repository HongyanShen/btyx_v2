package files_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/content"
	"baituber.cn/yanxuan/v2/internal/modules/files"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"github.com/shopspring/decimal"
)

type memoryStore struct {
	mu       sync.Mutex
	metadata objectstore.Metadata
	prefix   []byte
	copies   []objectstore.CopyRequest
	deleted  []objectstore.Reference
	copyErr  error
}

func (store *memoryStore) SignUpload(context.Context, objectstore.UploadRequest) (objectstore.UploadForm, error) {
	return objectstore.UploadForm{
		URL: "https://test-bucket.tos-cn-beijing.volces.com/",
	}, nil
}

func (store *memoryStore) Inspect(context.Context, objectstore.Reference) (objectstore.Metadata, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.metadata, nil
}

func (store *memoryStore) ReadPrefix(context.Context, objectstore.Reference, string) ([]byte, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([]byte(nil), store.prefix...), nil
}

func (store *memoryStore) Copy(_ context.Context, request objectstore.CopyRequest) (objectstore.Metadata, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.copies = append(store.copies, request)
	return objectstore.Metadata{
		ETag:      "final-etag",
		VersionID: "final-version",
	}, store.copyErr
}

func (store *memoryStore) Delete(_ context.Context, reference objectstore.Reference) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.deleted = append(store.deleted, reference)
	return nil
}

func (store *memoryStore) SignDownload(context.Context, objectstore.Reference, time.Duration) (string, error) {
	return "https://test-bucket.tos-cn-beijing.volces.com/private?signature=test", nil
}

func validStore() *memoryStore {
	return &memoryStore{
		metadata: objectstore.Metadata{
			Size:      8,
			MediaType: "image/png",
			ETag:      "staging-etag",
			VersionID: "staging-version",
		},
		prefix: []byte{137, 80, 78, 71, 13, 10, 26, 10},
	}
}

func uploadInput() files.InitiateInput {
	return files.InitiateInput{
		Name:      "图片.png",
		MediaType: "image/png",
		Size:      8,
	}
}

func TestDirectUploadRequiresServerVerificationAndOwner(t *testing.T) {
	services := testkit.Service(t, "../../../..")
	services.Config.Storage = config.ObjectStorage{
		Bucket: "test-bucket",
		Prefix: "btyx-v2/test/",
	}
	store := validStore()
	services.FilesService.Store = store
	ctx := context.Background()
	ticket, err := services.InitiateUpload(ctx, "admin", enum.RoleAdmin, uploadInput(), "direct-owner")
	if err != nil {
		t.Fatal(err)
	}

	if err := services.CheckOwnedFile(ctx, "admin", ticket.ID); err == nil {
		t.Fatal("pending upload became usable before verification")
	}

	if _, err := services.CreateContent(ctx, "admin", content.Input{
		Title: "未核验内容",
		Body:  "发布要求",
		Files: []string{ticket.ID},
	}); err == nil {
		t.Fatal("content accepted a pending upload")
	}

	if _, err := services.CreateTask(ctx, "admin", task.TaskInput{
		Title:     "未核验任务",
		Body:      "发布要求",
		PriceFen:  "750",
		Platforms: []enum.Platform{enum.PlatformDouyin},
		Files:     []string{ticket.ID},
	}); err == nil {
		t.Fatal("task version accepted a pending upload")
	}

	if _, err := services.CompleteUpload(ctx, "someone-else", ticket.ID); err == nil {
		t.Fatal("another user completed an upload")
	}

	completed, err := services.CompleteUpload(ctx, "admin", ticket.ID)
	if err != nil || completed.ID != ticket.ID {
		t.Fatalf("completion failed: %+v, %v", completed, err)
	}

	if _, err := services.CompleteUpload(ctx, "admin", ticket.ID); err != nil {
		t.Fatal(err)
	}

	record, err := services.Client.FileObject.Get(ctx, ticket.ID)
	if err != nil {
		t.Fatal(err)
	}

	if record.Status != enum.FileReady || record.ObjectKey == record.UploadKey || len(store.copies) != 1 {
		t.Fatal("completion did not freeze an independent verified object")
	}

	if store.copies[0].Source.VersionID != "staging-version" || store.copies[0].SourceETag != "staging-etag" {
		t.Fatal("copy did not bind the inspected source version and ETag")
	}
}

func TestConcurrentCompletionsKeepOnlyOneAcceptedObject(t *testing.T) {
	services := testkit.Service(t, "../../../..")
	services.Config.Storage = config.ObjectStorage{
		Bucket: "test-bucket",
		Prefix: "btyx-v2/test/",
	}
	store := validStore()
	services.FilesService.Store = store
	ctx := context.Background()
	ticket, err := services.InitiateUpload(ctx, "admin", enum.RoleAdmin, uploadInput(), "concurrent-upload")
	if err != nil {
		t.Fatal(err)
	}

	var wait sync.WaitGroup
	errors := make(chan error, 6)
	for index := 0; index < 6; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := services.CompleteUpload(ctx, "admin", ticket.ID)
			errors <- err
		}()
	}

	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}

	record, err := services.Client.FileObject.Get(ctx, ticket.ID)
	if err != nil {
		t.Fatal(err)
	}

	for _, deleted := range store.deleted {
		if deleted.Key == record.ObjectKey {
			t.Fatal("concurrent completion deleted the accepted object")
		}
	}

	if record.Status != enum.FileReady || len(store.deleted) != len(store.copies)-1 {
		t.Fatal("concurrent completion left multiple accepted candidates")
	}
}

func TestDownloadRequiresOwnerAdminOrAnActiveAssignmentSnapshot(t *testing.T) {
	services := testkit.Service(t, "../../../..")
	services.Config.Storage = config.ObjectStorage{
		Bucket: "test-bucket",
		Prefix: "btyx-v2/test/",
	}
	services.FilesService.Store = validStore()
	ctx := context.Background()
	ticket, err := services.InitiateUpload(ctx, "admin", enum.RoleAdmin, uploadInput(), "download-scope")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := services.CompleteUpload(ctx, "admin", ticket.ID); err != nil {
		t.Fatal(err)
	}

	login, err := services.DemoLogin(ctx, "file-reader")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := services.Download(ctx, login.UserID, enum.RoleUser, ticket.ID); err == nil {
		t.Fatal("unrelated user obtained a private download URL")
	}

	version, err := services.CreateTask(ctx, "admin", task.TaskInput{
		Title:     "私有素材权限",
		Body:      "发布要求",
		PriceFen:  "750",
		Platforms: []enum.Platform{enum.PlatformDouyin},
		Files:     []string{ticket.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	assigned, err := services.Client.Assignment.Create().
		SetUserID(login.UserID).
		SetTaskID(version.TaskID).
		SetVersionID(version.ID).
		SetPriceFen(decimal.NewFromInt(750)).
		SetPlatforms([]enum.Platform{enum.PlatformDouyin}).
		SetSource(enum.SourceAssigned).
		SetBusinessDate("2026-11-17").
		SetDueDate("2026-11-17").
		SetRequestKey("private-assignment").
		SetActorID("admin").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	download, err := services.Download(ctx, login.UserID, enum.RoleUser, ticket.ID)
	if err != nil {
		t.Fatal(err)
	}

	if download.Name != uploadInput().Name || download.MediaType != uploadInput().MediaType {
		t.Fatal("download ticket did not include preview metadata")
	}

	_, err = services.Client.Assignment.UpdateOneID(assigned.ID).SetStatus(enum.AssignmentRevoked).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := services.Download(ctx, login.UserID, enum.RoleUser, ticket.ID); err == nil {
		t.Fatal("revoked assignee obtained a new private URL")
	}
}

func TestDirectUploadRejectsSizeAndBodyTypeMismatch(t *testing.T) {
	for _, scenario := range []string{"size", "body", "copy"} {
		t.Run(scenario, func(t *testing.T) {
			services := testkit.Service(t, "../../../..")
			services.Config.Storage = config.ObjectStorage{
				Bucket: "test-bucket",
				Prefix: "btyx-v2/test/",
			}
			store := validStore()
			services.FilesService.Store = store
			ctx := context.Background()
			ticket, err := services.InitiateUpload(ctx, "admin", enum.RoleAdmin, uploadInput(), "direct-mismatch")
			if err != nil {
				t.Fatal(err)
			}

			switch scenario {
			case "size":
				store.metadata.Size++
			case "body":
				store.prefix = []byte("not an image")
			case "copy":
				store.copyErr = errors.New("source changed")
			}

			if _, err := services.CompleteUpload(ctx, "admin", ticket.ID); err == nil {
				t.Fatal("unverified object was accepted")
			}

			if err := services.CheckOwnedFile(ctx, "admin", ticket.ID); err == nil {
				t.Fatal("unverified object can be attached to business records")
			}
		})
	}
}

func TestUploadTicketIsIdempotentAndExpires(t *testing.T) {
	services := testkit.Service(t, "../../../..")
	services.Config.Storage = config.ObjectStorage{
		Bucket: "test-bucket",
		Prefix: "btyx-v2/test/",
	}
	services.FilesService.Store = validStore()
	ctx := context.Background()
	first, err := services.InitiateUpload(ctx, "admin", enum.RoleAdmin, uploadInput(), "same-ticket")
	if err != nil {
		t.Fatal(err)
	}

	second, err := services.InitiateUpload(ctx, "admin", enum.RoleAdmin, uploadInput(), "same-ticket")
	if err != nil || second.ID != first.ID {
		t.Fatal("retry created another upload")
	}

	changed := uploadInput()
	changed.Size++
	if _, err := services.InitiateUpload(ctx, "admin", enum.RoleAdmin, changed, "same-ticket"); err == nil {
		t.Fatal("changed payload reused an upload key")
	}

	now := services.Now()
	services.Now = func() time.Time { return now.Add(20 * time.Minute) }
	if _, err := services.CompleteUpload(ctx, "admin", first.ID); err == nil {
		t.Fatal("expired upload was completed")
	}
}
