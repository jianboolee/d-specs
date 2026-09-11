package idgen

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewIsUUIDv7(t *testing.T) {
	id := New()

	if len(id) != 36 {
		t.Fatalf("期望 36 字符的标准 UUID 写法，实际 %d 字符: %q", len(id), id)
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		t.Fatalf("不是合法 UUID: %v", err)
	}

	if version := parsed.Version(); version != 7 {
		t.Fatalf("期望版本 7，实际 %d", version)
	}
}

// UUID v7 的意义就在于时间有序：作为主键能让 B-tree 插入集中在右侧。
// 这个性质一旦被换成 v4 就没了，所以单独断言。
func TestNewIsTimeOrdered(t *testing.T) {
	const n = 50

	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		ids = append(ids, New())
		if i < n-1 {
			// UUID v7 的时间戳是毫秒精度，同毫秒内按随机位排序，
			// 因此要跨毫秒才能断言严格递增
			time.Sleep(2 * time.Millisecond)
		}
	}

	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("第 %d 个 ID 不大于前一个，失去时间有序性:\n  %s\n  %s", i, ids[i-1], ids[i])
		}
	}
}

func TestNewIsUnique(t *testing.T) {
	const n = 2000

	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := New()
		if _, dup := seen[id]; dup {
			t.Fatalf("生成重复 ID: %s", id)
		}
		seen[id] = struct{}{}
	}
}

// token 是「持有即可访问」的凭证，不该带时间戳：既泄露创建时间，
// 又让前缀可预测。这里断言它确实不是 UUID v7。
func TestTokenIsNotTimeOrdered(t *testing.T) {
	if len(Token()) != TokenLen {
		t.Fatalf("token 期望 %d 位，实际 %d 位", TokenLen, len(Token()))
	}
	if _, err := uuid.Parse(Token()); err == nil {
		t.Fatal("token 不该是 UUID —— 它需要纯随机，而不是带时间戳的有序 ID")
	}

	// 连续生成的 token 不应呈现递增趋势
	increasing := 0
	prev := Token()
	for i := 0; i < 200; i++ {
		cur := Token()
		if prev < cur {
			increasing++
		}
		prev = cur
	}
	if increasing > 160 {
		t.Fatalf("token 呈现有序倾向（%d/200 递增），可能误用了有序 ID 生成器", increasing)
	}
}

func TestTokenCharset(t *testing.T) {
	const allowed = "0123456789abcdefghijklmnopqrstuvwxyz"

	for i := 0; i < 50; i++ {
		token := Token()
		for _, r := range token {
			if !strings.ContainsRune(allowed, r) {
				t.Fatalf("token 含字母表外字符 %q: %s", r, token)
			}
		}
	}
}
