package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlattenFieldsData_AlreadyFlat(t *testing.T) {
	data := []MapStr{
		{
			"dataset.name": "name-1",
			"dataset.type": "type-1",
		},
		{
			"dataset.name": "name-2",
			"dataset.type": "type-2",
		},
	}

	flattened := flattenFieldsData(data)
	require.Len(t, flattened, 2)
	require.Equal(t, `{"dataset.name":"name-1","dataset.type":"type-1"}`, flattened[0].String())
	require.Equal(t, `{"dataset.name":"name-2","dataset.type":"type-2"}`, flattened[1].String())
}

func TestFlattenFieldsData_Object(t *testing.T) {
	data := []MapStr{
		{
			"dataset": MapStr{
				"name": "name-1",
				"type": "type-1",
			},
		},
	}

	flattened := flattenFieldsData(data)
	require.Len(t, flattened, 1)
	require.Equal(t, `{"dataset.name":"name-1","dataset.type":"type-1"}`, flattened[0].String())
}

func TestFlattenFieldsData_ObjectWithFields(t *testing.T) {
	data := []MapStr{
		{
			"dataset": MapStr{
				"fields": MapStr{
					"name": "name-1",
					"type": "type-1",
				},
			},
		},
	}

	flattened := flattenFieldsData(data)
	require.Len(t, flattened, 1)
	require.Equal(t, `{"dataset.name":"name-1","dataset.type":"type-1"}`, flattened[0].String())
}

