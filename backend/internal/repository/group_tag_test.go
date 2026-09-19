package repository

import (
	"strings"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/stretchr/testify/require"
)

func TestGroupEntityPreservesCustomTag(t *testing.T) {
	for _, tag := range []string{strings.Repeat("\u56fe", 20), strings.Repeat("\U0001f680", 20)} {
		t.Run(tag, func(t *testing.T) {
			entity := &dbent.Group{Tag: tag, TagColor: "#123456"}
			require.NoError(t, group.TagValidator(entity.Tag), "twenty Unicode codepoints must fit the Ent byte limit")
			got := groupEntityToService(entity)
			require.Equal(t, entity.Tag, got.Tag)
			require.Equal(t, entity.TagColor, got.TagColor)
		})
	}
}
