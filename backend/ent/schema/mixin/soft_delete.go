package mixin

import (
	"context"
	"fmt"

	gen "baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/hook"
	"baituber.cn/yanxuan/v2/ent/intercept"
	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

const deletedColumn = "deleted"

type SoftDelete struct{ mixin.Schema }

func (SoftDelete) Fields() []ent.Field {
	return []ent.Field{
		field.Bool(deletedColumn).Default(false),
	}
}

func (SoftDelete) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{
		intercept.TraverseFunc(func(_ context.Context, query intercept.Query) error {
			query.WhereP(NotDeleted)

			return nil
		}),
	}
}

func (SoftDelete) Hooks() []ent.Hook {
	return []ent.Hook{
		hook.On(softDeleteMutation, ent.OpDelete|ent.OpDeleteOne|ent.OpUpdate|ent.OpUpdateOne),
	}
}

func softDeleteMutation(next ent.Mutator) ent.Mutator {
	return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
		row, ok := mutation.(interface {
			SetOp(ent.Op)
			SetDeleted(bool)
			WhereP(...func(*sql.Selector))
			Client() *gen.Client
		})
		if !ok {
			return nil, fmt.Errorf("unexpected soft delete mutation %T", mutation)
		}

		if mutation.Op().Is(ent.OpDelete | ent.OpDeleteOne) {
			row.SetOp(ent.OpUpdate)
			row.SetDeleted(true)
			// The update builder applies the schema's update defaults, including updated_at.
			return row.Client().Mutate(ctx, mutation)
		}

		row.WhereP(NotDeleted)
		return next.Mutate(ctx, mutation)
	})
}

// NotDeleted also applies to SQL subqueries and edge predicates built without an Ent query.
func NotDeleted(selector *sql.Selector) {
	selector.Where(sql.IsFalse(selector.C(deletedColumn)))
}
