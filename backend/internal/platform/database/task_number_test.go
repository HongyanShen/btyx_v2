package database_test

import (
	"sync"
	"testing"

	"baituber.cn/yanxuan/v2/internal/platform/database"
	"baituber.cn/yanxuan/v2/internal/platform/displaynumber"
	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestNamedCountersPreserveExistingNumbers(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := t.Context()
	_, err := service.DB.ExecContext(ctx, `
		ALTER TABLE task_number_counters DROP COLUMN IF EXISTS name;
		INSERT INTO task_number_counters (id, created_at, last_number) VALUES
		    ('000000000000000000000000001', now(), 67),
		    ('000000000000000000000000002', now(), 14);
	`)
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatal(err)
		}
	}

	for _, counter := range []struct {
		key  displaynumber.Counter
		name string
		want int64
	}{
		{displaynumber.Task, "task", 68},
		{displaynumber.Nickname, "nickname", 15},
	} {
		next, err := counter.key.Next(ctx, service.Client)
		if err != nil || next != counter.want {
			t.Fatalf("%s next number = %d, want %d: %v", counter.name, next, counter.want, err)
		}

		var stored int64
		err = service.DB.QueryRowContext(ctx, "SELECT last_number FROM task_number_counters WHERE name = $1", counter.name).Scan(&stored)
		if err != nil || stored != next {
			t.Fatalf("counter %q stored %d, want %d: %v", counter.name, stored, next, err)
		}
	}
}

func TestCounterReturnsDistinctNumbersAcrossConcurrentCallers(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := t.Context()
	const callers = 24
	var wait sync.WaitGroup
	var numbers [callers]int64
	for i := range callers {
		wait.Go(func() {
			number, err := displaynumber.Task.Next(ctx, service.Client)
			if err != nil {
				t.Error(err)
			}

			numbers[i] = number
		})
	}

	wait.Wait()
	seen := make(map[int64]bool)
	for _, number := range numbers {
		if number < 1 || number > callers || seen[number] {
			t.Fatalf("unexpected concurrent allocation: %v", numbers)
		}

		seen[number] = true
	}

	if err := database.Migrate(ctx, service.Config); err != nil {
		t.Fatal(err)
	}

	next, err := displaynumber.Task.Next(ctx, service.Client)
	if err != nil || next != callers+1 {
		t.Fatalf("unused numbers were recycled: next=%d err=%v", next, err)
	}
}

func TestTaskNumbersBackfillAndRemainStable(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := t.Context()
	_, err := service.DB.ExecContext(ctx, `
		ALTER TABLE tasks DROP COLUMN display_number;
		INSERT INTO tasks (id, creator_id, created_at, updated_at)
		VALUES ('old-a', 'admin', now(), now()), ('old-b', 'admin', now(), now());
	`)
	if err != nil {
		t.Fatal(err)
	}

	var original int64
	for range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatal(err)
		}

		a, err := service.Client.Task.Get(ctx, "old-a")
		if err != nil {
			t.Fatal(err)
		}

		b, err := service.Client.Task.Get(ctx, "old-b")
		if err != nil {
			t.Fatal(err)
		}

		if a.DisplayNumber <= 0 || b.DisplayNumber <= 0 || a.DisplayNumber == b.DisplayNumber || (original != 0 && original != a.DisplayNumber) {
			t.Fatalf("unexpected migrated task numbers %d, %d (original %d)", a.DisplayNumber, b.DisplayNumber, original)
		}

		original = a.DisplayNumber
	}

	allocated, err := displaynumber.Task.Next(ctx, service.Client)
	if err != nil {
		t.Fatal(err)
	}

	var number int64
	err = service.DB.QueryRowContext(ctx, `
		INSERT INTO tasks (id, creator_id, created_at, updated_at, display_number)
		VALUES ('new', 'admin', now(), now(), $1) RETURNING display_number
	`, allocated).Scan(&number)
	if err != nil {
		t.Fatal(err)
	}

	if number <= 2 {
		t.Fatalf("new task did not receive a fresh number: %d", number)
	}
}

func TestCounterContinuesPastLegacySequenceGaps(t *testing.T) {
	service := testkit.Service(t, "../../../..")
	ctx := t.Context()
	_, err := service.DB.ExecContext(ctx, `
		CREATE SEQUENCE task_display_number_seq START WITH 40;
		CREATE FUNCTION next_task_display_number() RETURNS bigint
		LANGUAGE sql VOLATILE RETURN nextval('task_display_number_seq');
		ALTER TABLE tasks ALTER COLUMN display_number SET DEFAULT next_task_display_number();
		ALTER TABLE creative_tasks ALTER COLUMN display_number SET DEFAULT next_task_display_number();
		INSERT INTO tasks (id, creator_id, created_at, updated_at) VALUES ('old', 'admin', now(), now());
		SELECT nextval('task_display_number_seq');
	`)
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := database.Migrate(ctx, service.Config); err != nil {
			t.Fatal(err)
		}
	}

	old, err := service.Client.Task.Get(ctx, "old")
	if err != nil || old.DisplayNumber != 40 {
		t.Fatalf("old task number changed: %v, %v", old, err)
	}

	number, err := displaynumber.Task.Next(ctx, service.Client)
	if err != nil || number != 42 {
		t.Fatalf("new counter used %d, want 42: %v", number, err)
	}
}
