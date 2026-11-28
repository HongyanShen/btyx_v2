package task

import (
	"context"
	"fmt"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/task"
	"baituber.cn/yanxuan/v2/internal/platform/shortcode"
)

const taskCodePrefix = "RW-"

func DisplayCode(number int64) (string, error) {
	if number < 1 {
		return "", fmt.Errorf("invalid task display number %d", number)
	}

	suffix, err := shortcode.Encode(uint64(number))
	if err != nil {
		return "", err
	}

	return taskCodePrefix + suffix, nil
}

func taskCodes(ctx context.Context, client *ent.Client, ids []string) (map[string]string, error) {
	rows, err := client.Task.Query().
		Where(task.IDIn(ids...)).
		Select(task.FieldID, task.FieldDisplayNumber).
		All(ctx)
	if err != nil {
		return nil, err
	}

	codes := make(map[string]string, len(rows))
	for _, row := range rows {
		code, err := DisplayCode(row.DisplayNumber)
		if err != nil {
			return nil, err
		}

		codes[row.ID] = code
	}

	return codes, nil
}
