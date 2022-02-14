package validate

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContext_ExtractOperationType(t *testing.T) {

	var testCases = []struct {
		Ctx            context.Context //nolint: containedctx
		ExpectedOpType operationType
	}{
		{
			Ctx:            WithOperationRequest(context.Background()),
			ExpectedOpType: request,
		},
		{
			Ctx:            WithOperationResponse(context.Background()),
			ExpectedOpType: response,
		},
		{
			Ctx:            context.Background(),
			ExpectedOpType: none,
		},
		{
			Ctx:            context.WithValue(context.Background(), validateCtxKey("dummy"), "dummy val"),
			ExpectedOpType: none,
		},
		{
			Ctx:            context.WithValue(context.Background(), operationTypeKey, "dummy val"),
			ExpectedOpType: none,
		},
		{
			Ctx:            context.WithValue(context.Background(), operationTypeKey, operationType("dummy val")),
			ExpectedOpType: none,
		},
	}

	for idx, toPin := range testCases {
		tc := toPin
		t.Run(fmt.Sprintf("TestCase #%d", idx), func(t *testing.T) {
			t.Parallel()
			op := extractOperationType(tc.Ctx)
			assert.Equal(t, tc.ExpectedOpType, op)
		})
	}

}
