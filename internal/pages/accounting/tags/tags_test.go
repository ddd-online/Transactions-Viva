package tags

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
	"transactions/internal/store/storetest"
)

func newTestModel(t *testing.T) (*Model, *storetest.Fixture) {
	t.Helper()
	fixture := storetest.Open(t)
	m := New()
	m.ledgers.Reset([]domain.Ledger{fixture.Home, fixture.Travel})
	m.SetDataSource(fixture.Workspace, nil)
	return m, fixture
}

func TestNewDefaults(t *testing.T) {
	m, _ := newTestModel(t)
	if m.kind != kindExpense || m.selected != "" {
		t.Fatalf("the page starts on %v with %q chosen", m.kind, m.selected)
	}
	if got := len(m.categories[kindExpense]); got != 9 {
		t.Fatalf("expense categories = %d, want 9", got)
	}
	if got := len(m.categories[kindIncome]); got != 8 {
		t.Fatalf("income categories = %d, want 8", got)
	}
	if got := len(m.categories[kindTransfer]); got != 2 {
		t.Fatalf("transfer categories = %d, want 2", got)
	}
	if len(m.currentTags()) != 0 {
		t.Fatal("the page starts with tags shown")
	}
	selectCategory(t, m, "餐饮美食")
	if got := len(m.currentTags()); got != 10 {
		t.Fatalf("餐饮美食 tags = %d, want 10", got)
	}
}

func TestTypeSwitchClearsChoice(t *testing.T) {
	m, _ := newTestModel(t)
	selectCategory(t, m, "餐饮美食")
	m.selectKind(kindIncome)
	if m.kind != kindIncome || m.selected != "" {
		t.Fatalf("after switching: kind %v, chosen %q", m.kind, m.selected)
	}
	if len(m.currentTags()) != 0 {
		t.Fatal("the income type kept the expense category's tags")
	}

	// Clicking the type already shown leaves the chosen category alone.
	selectCategory(t, m, "工资奖金")
	m.selectKind(kindIncome)
	if m.selected != "工资奖金" {
		t.Fatalf("re-choosing the same type cleared %q", m.selected)
	}
}

func TestAddCategory(t *testing.T) {
	m, _ := newTestModel(t)
	m.categoryName = "餐饮美食"
	m.confirmCategory()
	if m.categoryError != "该分类已存在" {
		t.Fatalf("duplicate error = %q", m.categoryError)
	}

	m.categoryName = "宠物"
	m.confirmCategory()
	if m.categoryError != "" {
		t.Fatalf("adding failed: %q", m.categoryError)
	}
	if m.selected != "宠物" {
		t.Fatalf("the new category is not chosen: %q", m.selected)
	}
	if got := m.categories[kindExpense][len(m.categories[kindExpense])-1].name; got != "宠物" {
		t.Fatalf("the new category is at %q", got)
	}
	if m.categoryOpen {
		t.Fatal("the dialog stayed open")
	}
}

func TestAddTagNeedsACategory(t *testing.T) {
	m, _ := newTestModel(t)
	m.tagName = "洗澡"
	m.confirmTag()
	if len(m.tags[kindExpense]["洗澡"]) != 0 {
		t.Fatal("a tag was added without a category")
	}

	selectCategory(t, m, "餐饮美食")
	m.tagName = "三餐"
	m.confirmTag()
	if m.tagError != "该标签已存在" {
		t.Fatalf("duplicate error = %q", m.tagError)
	}

	m.tagName = "早餐"
	m.confirmTag()
	if m.tagError != "" {
		t.Fatalf("adding failed: %q", m.tagError)
	}
	tags := m.currentTags()
	if len(tags) == 0 || tags[len(tags)-1].name != "早餐" {
		t.Fatalf("the new tag is not at the end: %v", tags)
	}
}

func TestDeleteCategoryRemovesItsTags(t *testing.T) {
	m, _ := newTestModel(t)
	selectCategory(t, m, "餐饮美食")
	if len(m.currentTags()) == 0 {
		t.Fatal("餐饮美食 has no tags")
	}

	m.askDeleteCategory("餐饮美食")
	m.confirmDelete()
	if m.selected != "" {
		t.Fatalf("the deleted category is still chosen: %q", m.selected)
	}
	if _, ok := m.tags[kindExpense]["餐饮美食"]; ok {
		t.Fatal("the category's tags were left behind")
	}
	for _, cat := range m.categories[kindExpense] {
		if cat.name == "餐饮美食" {
			t.Fatal("the category was not removed")
		}
	}
}

func TestDeleteTag(t *testing.T) {
	m, _ := newTestModel(t)
	selectCategory(t, m, "餐饮美食")
	before := len(m.currentTags())
	m.askDeleteTag("咖啡")
	m.confirmDelete()
	if got := len(m.currentTags()); got != before-1 {
		t.Fatalf("tags = %d, want %d", got, before-1)
	}
	for _, one := range m.currentTags() {
		if one.name == "咖啡" {
			t.Fatal("the tag was not removed")
		}
	}
}

func TestMoveCategoryAndTag(t *testing.T) {
	m, _ := newTestModel(t)
	m.moveCategory([]int{0}, 3)
	categories := m.categories[kindExpense]
	if categories[2].name != "餐饮美食" {
		t.Fatalf("after the move: %v", categoryNames(categories))
	}
	m.moveCategory([]int{2}, 0)
	if categories := m.categories[kindExpense]; categories[0].name != "餐饮美食" {
		t.Fatalf("after moving back: %v", categoryNames(categories))
	}

	selectCategory(t, m, "餐饮美食")
	tags := m.currentTags()
	moved := tags[0].name
	m.moveTag([]int{0}, 2)
	tags = m.currentTags()
	if tags[1].name != moved {
		t.Fatalf("after the tag move: %v", tagNames(tags))
	}
	m.moveTag([]int{1}, 0)
	if tags := m.currentTags(); tags[0].name != moved {
		t.Fatalf("after moving the tag back: %v", tagNames(tags))
	}
}

func TestCloseOverlays(t *testing.T) {
	m := New()
	m.categoryOpen, m.categoryError = true, "x"
	m.tagOpen, m.tagError = true, "y"
	m.deleteOpen = true
	m.CloseOverlays()
	if m.categoryOpen || m.tagOpen || m.deleteOpen || m.categoryError != "" || m.tagError != "" {
		t.Fatal("CloseOverlays left a dialog open")
	}
}

// selectCategory chooses a category by name. The list's own choice is exercised
// by the driver test; here there is no paint, so the page state is set directly.
func selectCategory(t *testing.T, m *Model, name string) {
	t.Helper()
	for _, cat := range m.categories[m.kind] {
		if cat.name != name {
			continue
		}
		m.selected = name
		m.tagList.Select(-1)
		return
	}
	t.Fatalf("no category %q", name)
}

func categoryNames(categories []category) []string {
	names := make([]string, len(categories))
	for index, cat := range categories {
		names[index] = cat.name
	}
	return names
}

func tagNames(tags []tag) []string {
	names := make([]string, len(tags))
	for index, one := range tags {
		names[index] = one.name
	}
	return names
}

func TestLedgerSwitchClearsTheChosenCategory(t *testing.T) {
	m, fixture := newTestModel(t)
	selectCategory(t, m, "餐饮美食")
	m.ledgers.SetActive(fixture.Travel.ID)
	m.Reload()
	if m.selected != "" || len(m.currentTags()) != 0 {
		t.Fatalf("after the ledger switch: selected %q, tags %v", m.selected, m.currentTags())
	}
	if m.ledgerID != fixture.Travel.ID {
		t.Fatalf("ledger = %q", m.ledgerID)
	}
}

// TestRowDeleteKeysWaitForThePointer is the two columns' delete keys: a list nobody
// points at stays clean, and the key appears on the row under the pointer — the
// same rule the other rails follow.
func TestRowDeleteKeysWaitForThePointer(t *testing.T) {
	m, _ := newTestModel(t)
	page := ui.NewPage(ui.PageConfig{View: m.View})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1200, Height: 800}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	if hasHit(driver, "删除分类 餐饮美食") {
		t.Fatal("a category nobody points at shows its delete key")
	}
	if err := driver.Move(centre(t, driver, "餐饮美食")); err != nil {
		t.Fatal(err)
	}
	if !hasHit(driver, "删除分类 餐饮美食") {
		t.Fatal("the category's delete key did not appear under the pointer")
	}

	// Choose the category: its tags appear, and their keys wait for the pointer too.
	clickName(t, driver, "餐饮美食")
	if hasHit(driver, "删除标签 三餐") {
		t.Fatal("a tag nobody points at shows its delete key")
	}
	if err := driver.Move(centre(t, driver, "三餐")); err != nil {
		t.Fatal(err)
	}
	if !hasHit(driver, "删除标签 三餐") {
		t.Fatal("the tag's delete key did not appear under the pointer")
	}
}

// centre is the middle of the control named name.
func centre(t *testing.T, driver *app.Driver, name string) (float32, float32) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			b := hit.Bounds
			return b.X + b.Width/2, b.Y + b.Height/2
		}
	}
	t.Fatalf("no control named %q", name)
	return 0, 0
}

// clickName presses the middle of the control named name.
func clickName(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	x, y := centre(t, driver, name)
	if err := driver.Click(x, y); err != nil {
		t.Fatal(err)
	}
}

// hasHit reports whether a control with exactly this name is painted.
func hasHit(driver *app.Driver, name string) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			return true
		}
	}
	return false
}
