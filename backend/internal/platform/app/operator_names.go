package app

import (
	"context"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/operator"
)

type OperatorNames map[string]string

func (names OperatorNames) Label(id string) string {
	if label := names[id]; label != "" {
		return label
	}

	switch id {
	case "system", "worker":
		return "系统任务"
	case "admin":
		return "原共享管理员"
	default:
		return "—"
	}
}

func LookupOperatorNames(ctx context.Context, client *ent.Client, ids []string) (OperatorNames, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := client.Operator.Query().
		Where(operator.IDIn(ids...)).
		Select(operator.FieldID, operator.FieldUsername).
		All(ctx)
	if err != nil {
		return nil, err
	}

	names := make(OperatorNames, len(rows))
	for _, row := range rows {
		names[row.ID] = row.Username
	}

	return names, nil
}
