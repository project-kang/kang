package vm

import (
	"context"

	"github.com/project-kang/kang/pkg/types"
)

type Driver interface {
	Name() string
	Create(ctx context.Context, req types.CreateVMRequest) (types.VM, error)
	List(ctx context.Context) ([]types.VM, error)
	Delete(ctx context.Context, id string) error
}
