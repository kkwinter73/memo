// データの型とメモを保持するストア。データ層。
package main

import (
	"errors"
	"sync"
	"time"
)

// メモ一件を表すドメインモデル（アプリが扱う主要なデータ）jsonタグはフロントエンド側への配慮でつけてる
type Memo struct {
	ID        int       `json:"id"`    // Goのフィールドは大文字始まり（公開するため）だけど、JSONの世界は小文字/スネークケースが慣習。このギャップを埋めるのがJSONタグ。
	Title     string    `json:"title"` // 無しだと: {"ID": 1, "Title": "...", "CreatedAt": "..."} ← フィールド名そのまま使われる
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// メモが見つからなかったときに返すエラー
// いちいち各関数の中にエラーを書かない理由は
// err == nil / != nil は「エラーが起きたか」だけ
// 「どんなエラーか」は別途判別が要る
// 判別の手段として「文字列で比較する」より「値で比較する」方が壊れにくい
// パッケージレベルに ErrNotFound を1個置けば、値で比較できる
var ErrNotFound = errors.New("memo not found")

// MemoStore はメモを保持するインメモリストア
// メモを保管するデータ層の中核。
// メモの操作の役割だけで、httpのことは知らない
type MemoStore struct {
	mu     sync.Mutex    // 並行アクセス保護。httpサーバーはリクエストごとに別goroutineでハンドラを動かすため競合しないように、MuteXで守る
	memos  map[int]*Memo // メモの置き場。キーがID。値がメモへのポインタ。ポインタなのはUpdateで中身を書き換えた時にマップの値も更新されるから
	nextID int           // 次に採番するＩＤ。オンメモリなので手動管理。ＤＢなら自動採番になる。
}

//全部小文字始まり(非公開)の理由
// 外から直接 store.memos[...] = ... されると、Mutex を経由しない書き込みで並行クラッシュが起きる。「触れるのは Create/Get/Update/Delete のメソッドだけ」という制約をかけたい。
// カプセル化: 外に見せるのはメソッド、内部状態(フィールド)は隠す。
// C#の private と同じ発想。
// mu を先頭に置くのは慣習
//「Mutexは、それが守るフィールドの直前に書く」がGoの流儀。
// 守る対象(memos、nextID)の真上にある、と一目でわかるように。

// NewMemoStoreは初期化済みの MemoStoreを返す
// マップのゼロ値は nil で、nilマップには書き込めない。
// だから make(map[int]*Memo) で明示的に初期化する必要がある。
// この初期化を1箇所に集約して、呼び出し側が間違えないようにする関数が NewMemoStore。
func NewMemoStore() *MemoStore {
	return &MemoStore{
		memos:  make(map[int]*Memo),
		nextID: 1,
	}
}

// Createで新しいメモを作る // ID と日時はサーバー側で決めたいから引数で受け取らない。クライアントがIDを指定できると採番が壊れる。DTOの話と同じ発想。
// 新しいメモを作って保管庫に入れる仕事。ID採番・時刻の付与・並行保護まで面倒を見る。
// ポインタレシーバ *MemoStore の理由 : s.nextID++ で内部状態を書き換えたいから。値レシーバだとコピーに対してインクリメントするだけで元のstoreに反映されない。
func (s *MemoStore) Create(title, content string) *Memo {
	s.mu.Lock() // Lock → defer Unlock の型マップへの書き込みとnextIDの変更を、他のgoroutineから守るため。defer でUnlockするのは「return で抜けても、パニックしても必ず解放される」保証のため。Goの鉄板イディオム
	defer s.mu.Unlock()

	now := time.Now() //now := time.Now() を変数に入れる理由 : CreatedAt と UpdatedAt を同じ時刻に揃えるため。毎回 time.Now() を呼ぶと微妙にずれる(数ナノ秒〜)。それに、後で読む人が「同じ時刻だな」と一目でわかる。
	memo := &Memo{
		ID:        s.nextID,
		Title:     title,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.memos[s.nextID] = memo
	s.nextID++
	return memo //呼び出し側(ハンドラ)が「作ったメモ」を受け取ってJSONレスポンスに使う。マップに入れたのと同じポインタを返すので、呼び出し側もマップの中身と同じ実体を参照する
}

// すべてのめもを返す
func (s *MemoStore) GetAll() []*Memo {
	s.mu.Lock()
	defer s.mu.Unlock()

	//スライスの容量を事前確保。長さ0で始めて、これからappendで追加していくけど、
	// 最終的に何個入るかはマップのサイズでわかってるから、その分のメモリを先に取っておく。
	// append時の再確保が起きないので効率的。
	result := make([]*Memo, 0, len(s.memos))

	// マップのrangeは key, value のペアを返す。キー(ID)は値の中にあるので不要 → _ で捨てる。
	// Goは未使用変数があるとコンパイルエラーになるので、使わないなら明示的に捨てる必要がある。
	for _, m := range s.memos {
		result = append(result, m)
	}

	return result
}

// 特定のメモを返す
func (s *MemoStore) Get(id int) (*Memo, error) {
	//Goの定石。「値とエラーのペアを返す」のが基本形。成功時は (memo, nil)、
	// 失敗時は (nil, err)。C#の try-catch の代わりに、戻り値でエラーを伝えるのがGoの流儀。
	s.mu.Lock()
	defer s.mu.Unlock()

	//マップアクセスの2値受け。
	// 1値受け memo := s.memos[id] → 存在しないキーのときゼロ値(今回なら nil)が返る。「存在しない」と「存在するけど値がnil」の区別がつかない。
	// 2値受け memo, ok := s.memos[id] → 第2戻り値 ok が「存在したか」のbool。
	//
	// マップで存在確認が必要なときはほぼ必ずこの形。

	memo, ok := s.memos[id]

	if !ok {
		return nil, ErrNotFound //  対称性
	}

	return memo, nil // 対称性
}

// 特定のメモを更新する
func (s *MemoStore) Update(id int, title string, content string) (*Memo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	memo, ok := s.memos[id]

	if !ok {
		return nil, ErrNotFound
	}

	memo.Title = title
	memo.Content = content
	memo.UpdatedAt = time.Now()
	return memo, nil
}

// 指定したIDのメモを削除する
func (s *MemoStore) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.memos[id]; !ok {
		return ErrNotFound
	}

	// 組み込み関数（map,id）
	delete(s.memos, id)
	return nil
}
