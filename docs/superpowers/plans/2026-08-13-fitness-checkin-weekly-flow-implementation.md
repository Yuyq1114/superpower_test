# Weekly Fitness Plan Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the date-based, three-dropdown workout flow with an atomically saved weekly plan editor and a direct “today’s workout” check-in experience.

**Architecture:** Add a backward-compatible weekly aggregate contract to plan-service while retaining stable plan/day/item IDs and legacy item lookup for checkin-service. PostgreSQL performs a versioned transactional migration from dates to ISO weekdays; new create/update operations persist the complete plan document in one transaction with optimistic locking and idempotency fingerprints. React uses one weekly-plan editor and one shared today-workouts query consumed by both Dashboard and Checkin.

**Tech Stack:** Go, Gin, gRPC/protobuf, GORM, PostgreSQL 16, React 19, TypeScript, React Router 7, TanStack Query, React Hook Form, Zod, Vitest, MSW, Playwright.

## Global Constraints

- ISO weekdays are integers `1=Monday` through `7=Sunday`.
- An `active` plan must have at least one weekday, and every weekday must contain at least one valid workout item.
- A workout item is valid only when its name is non-empty, numeric fields are non-negative, and at least one of sets, repetitions, or duration is positive.
- Existing item IDs must survive aggregate edits; checkin-service and historical check-ins continue to reference `workout_item_id`.
- New aggregate responses use `{ "weekly_plan": { "plan": {}, "days": [] } }`.
- Nested validation errors use optional `field_errors: Record<string, string>` in the existing error envelope.
- Aggregate HTTP request bodies remain limited to 1 MiB and return HTTP 413 when exceeded.
- Weekly plan creation requires `Idempotency-Key`; the server stores a canonical SHA-256 request fingerprint and returns 409 when the same key is reused with different content.
- Weekly plan updates require `expected_updated_at`; stale writes return 409.
- The weekly data migration runs in a PostgreSQL transaction, records a migration version, and fails closed on duplicate weekdays without silently merging data.
- Deployment stops the old plan-service before migration; old date writers and new weekday writers must not run concurrently.
- The legacy `POST /plans` creates `draft`; legacy update cannot activate an incomplete plan.
- Do not add a shared action library, reusable workout templates, multi-week cycles, drag sorting, or per-set completion tracking.

---

### Task 1: Restore the Legacy Baseline and Define the Weekly Proto Contract

**Files:**
- Modify: `services/plan-service/internal/service/plan_test.go`
- Modify: `services/plan-service/internal/service/plan.go`
- Modify: `proto/plan/v1/plan.proto`
- Regenerate: `proto/gen/plan/v1/plan.pb.go`
- Regenerate: `proto/gen/plan/v1/plan_grpc.pb.go`
- Modify: `services/plan-service/internal/identity/context_test.go`
- Modify: `frontend/src/features/plans/plans.test.tsx`

**Interfaces:**
- Produces RPCs:
  - `CreateWeeklyPlan(CreateWeeklyPlanRequest) returns (WeeklyPlanResponse)`
  - `GetWeeklyPlan(GetWeeklyPlanRequest) returns (WeeklyPlanResponse)`
  - `ReplaceWeeklyPlan(ReplaceWeeklyPlanRequest) returns (WeeklyPlanResponse)`
- Preserves every existing message field number and `GetWorkoutItem` behavior.
- Produces `WeeklyPlanDocument`, `WeeklyWorkout`, `WeeklyWorkoutInput`, and `WeeklyItemInput`.

- [ ] **Step 1: Replace the current regression test with the approved legacy behavior**

Add this test to `services/plan-service/internal/service/plan_test.go`:

```go
func TestCreatePlanStartsDraftUntilScheduleIsComplete(t *testing.T) {
	plan, err := New(fakeRepo{}).CreatePlan(context.Background(), "u", CreatePlanInput{Name: "力量训练"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "draft" {
		t.Fatalf("CreatePlan() status = %q, want draft", plan.Status)
	}
}
```

Remove `TestCreatePlanStartsActiveSoItIsImmediatelyAvailableForCheckin`.

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
go test ./services/plan-service/internal/service -run TestCreatePlanStartsDraftUntilScheduleIsComplete -count=1
```

Expected: FAIL with `status = "active", want draft`.

- [ ] **Step 3: Restore legacy plan creation to draft**

Change the `model.Plan` literal in `Service.CreatePlan` to:

```go
p := model.Plan{
	ID:             uuid.NewString(),
	UserID:         u,
	Name:           in.Name,
	Status:         "draft",
	IdempotencyKey: in.IdempotencyKey,
	CreatedAt:      now,
	UpdatedAt:      now,
}
```

- [ ] **Step 4: Run the focused test and verify GREEN**

Run the Step 2 command.

Expected: PASS.

Also restore the fake Gateway’s legacy `POST /plans` response in `frontend/src/features/plans/plans.test.tsx` from `status: "active"` to `status: "draft"` so frontend compatibility tests model the approved contract.

- [ ] **Step 5: Add the weekly aggregate protobuf messages and RPCs**

Append messages with new field numbers; do not renumber existing messages:

```proto
message WeeklyItemInput {
  string id = 1;
  string name = 2;
  int32 sets = 3;
  int32 repetitions = 4;
  double weight = 5;
  int32 duration_seconds = 6;
}

message WeeklyWorkoutInput {
  string id = 1;
  int32 weekday = 2;
  string title = 3;
  repeated WeeklyItemInput items = 4;
}

message WeeklyPlanInput {
  string name = 1;
  string status = 2;
  repeated WeeklyWorkoutInput days = 3;
}

message WeeklyWorkout {
  string id = 1;
  string plan_id = 2;
  int32 weekday = 3;
  string title = 4;
  repeated WorkoutItem items = 5;
  string created_at = 6;
  string updated_at = 7;
}

message WeeklyPlanDocument {
  Plan plan = 1;
  repeated WeeklyWorkout days = 2;
}

message CreateWeeklyPlanRequest {
  string user_id = 1;
  string idempotency_key = 2;
  WeeklyPlanInput weekly_plan = 3;
}

message GetWeeklyPlanRequest {
  string user_id = 1;
  string plan_id = 2;
}

message ReplaceWeeklyPlanRequest {
  string user_id = 1;
  string plan_id = 2;
  string expected_updated_at = 3;
  WeeklyPlanInput weekly_plan = 4;
}

message WeeklyPlanResponse {
  WeeklyPlanDocument weekly_plan = 1;
}
```

Add these methods to `PlanService`:

```proto
rpc CreateWeeklyPlan(CreateWeeklyPlanRequest) returns (WeeklyPlanResponse);
rpc GetWeeklyPlan(GetWeeklyPlanRequest) returns (WeeklyPlanResponse);
rpc ReplaceWeeklyPlan(ReplaceWeeklyPlanRequest) returns (WeeklyPlanResponse);
```

- [ ] **Step 6: Regenerate protobuf code**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/generate-proto.ps1
```

Expected: `Generated protobuf files in ...\proto\gen`.

- [ ] **Step 7: Extend identity coverage before implementing RPC handlers**

Add the three requests to `TestAllPlanRequestsEnforceUserMatch`:

```go
&planv1.CreateWeeklyPlanRequest{UserId: "other"},
&planv1.GetWeeklyPlanRequest{UserId: "other"},
&planv1.ReplaceWeeklyPlanRequest{UserId: "other"},
```

Run:

```powershell
go test ./proto/gen/plan/v1 ./services/plan-service/internal/identity -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit Task 1**

```powershell
git add proto/plan/v1/plan.proto proto/gen/plan/v1 services/plan-service/internal/service/plan.go services/plan-service/internal/service/plan_test.go services/plan-service/internal/identity/context_test.go frontend/src/features/plans/plans.test.tsx
git commit -m "feat: define weekly plan aggregate contract"
```

---

### Task 2: Add the Versioned Transactional Weekday Migration

**Files:**
- Modify: `services/plan-service/internal/model/model.go`
- Create: `services/plan-service/internal/repository/weekly_migration.go`
- Create: `services/plan-service/internal/repository/weekly_migration_test.go`
- Modify: `services/plan-service/internal/repository/repository.go`
- Modify: `services/plan-service/internal/repository/integration_test.go`
- Modify: `services/plan-service/internal/repository/migration_contract_test.go`
- Modify: `services/plan-service/internal/service/plan.go`
- Modify: `services/plan-service/internal/grpc/server.go`
- Modify: `deploy/config_test.go`

**Interfaces:**
- Produces `func migrateWeeklySchedule(ctx context.Context, db *gorm.DB, schema string) error`.
- Produces `type WeeklyMigrationConflictError struct { Conflicts []WeeklyConflict }`.
- Changes `WorkoutDay.Date` to nullable `*time.Time` and adds `Weekday int` and `Title string`.
- Adds plan `RequestFingerprint string`.

- [ ] **Step 1: Isolate PostgreSQL integration tests**

Add `//go:build integration` to `integration_test.go`, and use `TEST_DATABASE_ADMIN_DSN` to create and drop a randomly named schema as already done by profile/statistics integration suites.

The setup must return:

```go
func integrationRepo(t *testing.T) (GORM, *gorm.DB) {
	t.Helper()
	adminDSN := os.Getenv("TEST_DATABASE_ADMIN_DSN")
	if adminDSN == "" {
		t.Fatal("TEST_DATABASE_ADMIN_DSN is required")
	}
	schema := "plan_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	admin := openPostgres(t, adminDSN)
	if err := admin.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`).Error
	})
	return GORM{DB: admin, Schema: schema}, admin
}
```

- [ ] **Step 2: Write migration RED tests**

Create tests that establish the old schema, insert data, call `migrateWeeklySchedule`, and assert:

```go
func TestWeeklyMigrationBackfillsISOWeekdayAndDemotesIncompleteActivePlan(t *testing.T) {
	repo, db := integrationRepo(t)
	createLegacyPlanSchema(t, db, repo.Schema)
	insertLegacyPlanDayAndItem(t, db, repo.Schema, "complete", "2026-08-10")
	insertLegacyEmptyActivePlan(t, db, repo.Schema, "empty")

	if err := migrateWeeklySchedule(context.Background(), db, repo.Schema); err != nil {
		t.Fatal(err)
	}

	assertDayWeekday(t, db, repo.Schema, "complete", 1)
	assertPlanStatus(t, db, repo.Schema, "complete", "active")
	assertPlanStatus(t, db, repo.Schema, "empty", "draft")
}

func TestWeeklyMigrationRejectsDuplicateMappedWeekdaysWithoutPartialWrites(t *testing.T) {
	repo, db := integrationRepo(t)
	createLegacyPlanSchema(t, db, repo.Schema)
	insertLegacyPlanDayAndItem(t, db, repo.Schema, "p1", "2026-08-10")
	insertLegacyPlanDayAndItem(t, db, repo.Schema, "p1", "2026-08-17")

	err := migrateWeeklySchedule(context.Background(), db, repo.Schema)
	var conflict *WeeklyMigrationConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want WeeklyMigrationConflictError", err)
	}
	if len(conflict.Conflicts) != 1 || conflict.Conflicts[0].PlanID != "p1" || conflict.Conflicts[0].Weekday != 1 {
		t.Fatalf("conflicts = %#v", conflict.Conflicts)
	}
	assertColumnMissing(t, db, repo.Schema, "workout_days", "weekday")
}
```

- [ ] **Step 3: Run migration tests and verify RED**

Run:

```powershell
$env:TEST_DATABASE_ADMIN_DSN='postgres://fitness:postgres-local-only@127.0.0.1:5432/fitness?sslmode=disable'
go test -tags=integration ./services/plan-service/internal/repository -run WeeklyMigration -count=1
```

Expected: FAIL because weekly migration types/functions do not exist.

- [ ] **Step 4: Extend models**

Use:

```go
type Plan struct {
	ID                 string `gorm:"column:id;primaryKey"`
	UserID             string `gorm:"column:user_id;index"`
	IdempotencyKey     string `gorm:"column:idempotency_key"`
	RequestFingerprint string `gorm:"column:request_fingerprint"`
	Name               string
	Status             string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type WorkoutDay struct {
	ID             string     `gorm:"column:id;primaryKey"`
	UserID         string     `gorm:"column:user_id;index"`
	PlanID         string     `gorm:"column:plan_id;index"`
	IdempotencyKey string     `gorm:"column:idempotency_key"`
	Date           *time.Time `gorm:"column:workout_date"`
	Weekday        int        `gorm:"column:weekday"`
	Title          string     `gorm:"column:title"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
```

- [ ] **Step 5: Implement the migration in one PostgreSQL transaction**

`migrateWeeklySchedule` must:

1. Return immediately when migration version `2026081301` exists.
2. Start `db.Transaction`.
3. Create `schema_migrations(version bigint primary key, applied_at timestamptz not null)`.
4. Add nullable `weekday`, `title`, `request_fingerprint`; drop `workout_date NOT NULL`.
5. Backfill `weekday = EXTRACT(ISODOW FROM workout_date)`.
6. Query grouped duplicate weekdays and return `WeeklyMigrationConflictError` containing plan ID, weekday, and ordered record IDs.
7. Demote active plans with no days or any day without an item.
8. Drop `plan_days_unique`.
9. Add weekday range check and unique `(user_id, plan_id, weekday)` constraint.
10. Insert migration version only after every statement succeeds.

Use identifier quoting through the existing validated schema path; values remain bound parameters.

- [ ] **Step 6: Wire migration after base table creation**

At the end of `migrateSchema`:

```go
if err := migrateWeeklySchedule(ctx, db, schema); err != nil {
	return fmt.Errorf("weekly schedule migration: %w", err)
}
return nil
```

Update the base `CREATE TABLE` statements so new installations include nullable `workout_date`, `weekday`, `title`, and `request_fingerprint` before the versioned migration applies constraints.

- [ ] **Step 7: Keep legacy date RPCs compiling and dual-populate weekday**

The compatibility `AddWorkoutDay` and `UpdateWorkoutDay` paths must derive ISO weekday from the supplied date and set both fields:

```go
func isoWeekday(t time.Time) int {
	weekday := int(t.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

date := in.Date.UTC()
d.Date = &date
d.Weekday = isoWeekday(date)
```

The gRPC `day` mapper returns an empty legacy date only when `Date == nil`; otherwise it formats `Date.Format("2006-01-02")`.

Extend `deploy/config_test.go` so every plan repository integration test file that opens PostgreSQL requires `//go:build integration`.

- [ ] **Step 8: Verify migration and migration contracts**

Run:

```powershell
go test -tags=integration ./services/plan-service/internal/repository -count=1
go test ./services/plan-service/internal/repository -run TestMigrationSource -count=1
go test ./deploy -run IntegrationSuitesHaveBuildTag -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit Task 2**

```powershell
git add services/plan-service/internal/model/model.go services/plan-service/internal/repository services/plan-service/internal/service/plan.go services/plan-service/internal/grpc/server.go deploy/config_test.go
git commit -m "feat: migrate workout dates to weekly schedules"
```

---

### Task 3: Implement Atomic Weekly Aggregate Persistence

**Files:**
- Create: `services/plan-service/internal/model/weekly_plan.go`
- Create: `services/plan-service/internal/repository/weekly_plan.go`
- Create: `services/plan-service/internal/repository/weekly_plan_test.go`
- Modify: `services/plan-service/internal/repository/integration_test.go`

**Interfaces:**
- Produces model aggregates:

```go
type WeeklyWorkout struct {
	Day   WorkoutDay
	Items []WorkoutItem
}

type WeeklyPlan struct {
	Plan Plan
	Days []WeeklyWorkout
}

```

- Produces these `GORM` methods; Task 4 defines the consumer-side service interface they satisfy:

```go
func (r GORM) CreateWeeklyPlan(context.Context, *model.WeeklyPlan) error
func (r GORM) GetWeeklyPlan(context.Context, string, string) (model.WeeklyPlan, error)
func (r GORM) ReplaceWeeklyPlan(context.Context, string, string, time.Time, *model.WeeklyPlan) error
func (r GORM) HasCompleteSchedule(context.Context, string, string) (bool, error)
```

- [ ] **Step 1: Write integration RED tests for atomicity and ID preservation**

Add:

```go
func TestCreateWeeklyPlanRollsBackEveryTableWhenAnItemInsertFails(t *testing.T) {
	repo, db := integrationRepo(t)
	if err := MigrateSchema(context.Background(), db, repo.Schema); err != nil {
		t.Fatal(err)
	}
	doc := validWeeklyPlan("p1", "u1")
	duplicate := doc.Days[0].Items[0]
	doc.Days[0].Items = append(doc.Days[0].Items, duplicate)

	err := repo.CreateWeeklyPlan(context.Background(), &doc)
	if err == nil {
		t.Fatal("expected insert failure")
	}
	assertTableCount(t, db, repo.Schema, "plans", 0)
	assertTableCount(t, db, repo.Schema, "workout_days", 0)
	assertTableCount(t, db, repo.Schema, "workout_items", 0)
}

func TestReplaceWeeklyPlanPreservesRetainedIDsAndRejectsStaleTimestamp(t *testing.T) {
	repo, db := integrationRepo(t)
	if err := MigrateSchema(context.Background(), db, repo.Schema); err != nil {
		t.Fatal(err)
	}
	doc := validWeeklyPlan("p1", "u1")
	if err := repo.CreateWeeklyPlan(context.Background(), &doc); err != nil {
		t.Fatal(err)
	}
	expected := doc.Plan.UpdatedAt
	retainedItemID := doc.Days[0].Items[0].ID
	doc.Days[0].Items[0].Weight = 62.5

	if err := repo.ReplaceWeeklyPlan(context.Background(), "u1", "p1", expected, &doc); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetWeeklyPlan(context.Background(), "u1", "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Days[0].Items[0].ID != retainedItemID {
		t.Fatalf("item ID changed: %s", got.Days[0].Items[0].ID)
	}
	if err := repo.ReplaceWeeklyPlan(context.Background(), "u1", "p1", expected, &doc); apperror.CodeOf(err) != apperror.CodeConflict {
		t.Fatalf("stale replace error = %v", err)
	}
}
```

Define the shared fixture in the same test file:

```go
func validWeeklyPlan(planID, userID string) model.WeeklyPlan {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return model.WeeklyPlan{
		Plan: model.Plan{
			ID:                 planID,
			UserID:             userID,
			IdempotencyKey:     "key-" + planID,
			RequestFingerprint: "fingerprint-" + planID,
			Name:               "每周力量训练",
			Status:             "active",
			CreatedAt:          now,
			UpdatedAt:          now,
		},
		Days: []model.WeeklyWorkout{{
			Day: model.WorkoutDay{
				ID:        "day-" + planID,
				UserID:    userID,
				PlanID:    planID,
				Weekday:   1,
				Title:     "胸肩",
				CreatedAt: now,
				UpdatedAt: now,
			},
			Items: []model.WorkoutItem{{
				ID:              "item-" + planID,
				UserID:          userID,
				WorkoutDayID:    "day-" + planID,
				Name:            "卧推",
				Sets:            4,
				Repetitions:     8,
				Weight:          60,
				DurationSeconds: 0,
				CreatedAt:       now,
				UpdatedAt:       now,
			}},
		}},
	}
}
```

- [ ] **Step 2: Run aggregate repository tests and verify RED**

Run:

```powershell
go test -tags=integration ./services/plan-service/internal/repository -run 'CreateWeeklyPlan|ReplaceWeeklyPlan' -count=1
```

Expected: FAIL because aggregate models and methods do not exist.

- [ ] **Step 3: Implement aggregate loading**

`GetWeeklyPlan` must:

- query plan with `user_id` and `plan_id`;
- query days ordered by `weekday`;
- query all items for those day IDs ordered by `created_at`;
- group items by `workout_day_id`;
- return empty slices rather than nil slices.

Use one plan query, one day query, and one item query; do not issue one item query per weekday.

- [ ] **Step 4: Implement idempotent atomic creation**

Inside `DB.Transaction`:

```go
result := tx.Table(table(r.Schema, "plans")).Clauses(clause.OnConflict{
	Columns:     []clause.Column{{Name: "user_id"}, {Name: "idempotency_key"}},
	DoNothing:   true,
	TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "idempotency_key <> ''"}}},
}).Create(&doc.Plan)
```

When `RowsAffected == 0`, load the existing aggregate by `(user_id, idempotency_key)`. Return `apperror.Conflict("idempotency key reused with different request")` when fingerprints differ; otherwise replace `*doc` with the existing aggregate and return nil.

Insert days and items only after a new plan row is confirmed. Any insert error must leave all three tables unchanged.

- [ ] **Step 5: Implement optimistic aggregate replacement**

Within one transaction:

1. Update plan with `WHERE user_id=? AND id=? AND updated_at=?`.
2. Return `apperror.Conflict("weekly plan was modified")` when zero rows are affected.
3. Load current day/item IDs under the same transaction.
4. Reject any client-supplied ID not owned by the same aggregate.
5. Update retained rows, create rows with empty IDs after assigning UUIDs, and delete only IDs omitted from the request.
6. Delete items before their parent days.
7. Set one new `updated_at` value across plan and modified child rows.

Do not use `Save` for the optimistic plan update.

- [ ] **Step 6: Implement completeness lookup for the legacy status path**

`HasCompleteSchedule` returns true only when:

```sql
EXISTS (SELECT 1 FROM workout_days WHERE user_id = ? AND plan_id = ? AND weekday BETWEEN 1 AND 7)
AND NOT EXISTS (
  SELECT 1
  FROM workout_days d
  WHERE d.user_id = ? AND d.plan_id = ?
    AND NOT EXISTS (
      SELECT 1 FROM workout_items i
      WHERE i.user_id = d.user_id AND i.workout_day_id = d.id
    )
)
```

- [ ] **Step 7: Verify repository package**

Run:

```powershell
go test -tags=integration ./services/plan-service/internal/repository -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit Task 3**

```powershell
git add services/plan-service/internal/model/weekly_plan.go services/plan-service/internal/repository/weekly_plan.go services/plan-service/internal/repository/weekly_plan_test.go services/plan-service/internal/repository/integration_test.go
git commit -m "feat: persist weekly plans atomically"
```

---

### Task 4: Add Weekly Validation, gRPC Mapping, and Structured Field Errors

**Files:**
- Modify: `pkg/apperror/error.go`
- Modify: `pkg/apperror/error_test.go`
- Create: `services/plan-service/internal/service/weekly_plan.go`
- Create: `services/plan-service/internal/service/weekly_plan_test.go`
- Modify: `services/plan-service/internal/service/plan.go`
- Modify: `services/plan-service/internal/grpc/server.go`
- Create: `services/plan-service/internal/grpc/weekly_plan_test.go`

**Interfaces:**
- Produces:

```go
type FieldError struct {
	Path    string
	Message string
}

func InvalidFields(message string, fields []FieldError) error
func FieldErrors(err error) []FieldError
```

- Produces the consumer-side repository interface:

```go
type WeeklyPlanRepository interface {
	CreateWeeklyPlan(context.Context, *model.WeeklyPlan) error
	GetWeeklyPlan(context.Context, string, string) (model.WeeklyPlan, error)
	ReplaceWeeklyPlan(context.Context, string, string, time.Time, *model.WeeklyPlan) error
	HasCompleteSchedule(context.Context, string, string) (bool, error)
}
```

- Produces service methods:

```go
func (s *Service) CreateWeeklyPlan(context.Context, string, string, WeeklyPlanInput) (model.WeeklyPlan, error)
func (s *Service) GetWeeklyPlan(context.Context, string, string) (model.WeeklyPlan, error)
func (s *Service) ReplaceWeeklyPlan(context.Context, string, string, time.Time, WeeklyPlanInput) (model.WeeklyPlan, error)
```

- [ ] **Step 1: Write service RED tests for nested validation and fingerprints**

Create table-driven tests asserting exact paths:

```go
func TestValidateWeeklyPlanReturnsStableFieldPaths(t *testing.T) {
	input := WeeklyPlanInput{
		Name: "",
		Days: []WeeklyWorkoutInput{
			{Weekday: 1, Items: nil},
			{Weekday: 1, Items: []WorkoutItemInput{{Name: "", Sets: -1}}},
		},
	}
	err := validateWeeklyPlan(input)
	if apperror.CodeOf(err) != apperror.CodeInvalidArgument {
		t.Fatalf("code = %s", apperror.CodeOf(err))
	}
	got := apperror.FieldErrors(err)
	want := []apperror.FieldError{
		{Path: "name", Message: "请输入计划名称"},
		{Path: "days.0.items", Message: "请至少添加一个训练项目"},
		{Path: "days.1.weekday", Message: "同一星期只能安排一次"},
		{Path: "days.1.items.0.name", Message: "请输入训练项目名称"},
		{Path: "days.1.items.0.sets", Message: "不能为负数"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatal(diff)
	}
}

func TestWeeklyPlanFingerprintIsStable(t *testing.T) {
	a := validWeeklyInput()
	b := validWeeklyInput()
	if weeklyPlanFingerprint(a) != weeklyPlanFingerprint(b) {
		t.Fatal("same document produced different fingerprints")
	}
	b.Days[0].Items[0].Weight++
	if weeklyPlanFingerprint(a) == weeklyPlanFingerprint(b) {
		t.Fatal("different documents produced same fingerprint")
	}
}
```

- [ ] **Step 2: Run service tests and verify RED**

Run:

```powershell
go test ./services/plan-service/internal/service -run 'ValidateWeeklyPlan|WeeklyPlanFingerprint' -count=1
```

Expected: FAIL because weekly validation is missing.

- [ ] **Step 3: Add structured application validation errors**

Extend `apperror.Error` with a private field slice and expose constructors/accessors:

```go
type Error struct {
	Code    Code
	Message string
	cause   error
	fields  []FieldError
}

type FieldError struct {
	Path    string
	Message string
}

func InvalidFields(message string, fields []FieldError) error {
	copyOfFields := append([]FieldError(nil), fields...)
	return &Error{Code: CodeInvalidArgument, Message: message, fields: copyOfFields}
}

func FieldErrors(err error) []FieldError {
	var appErr *Error
	if !As(err, &appErr) {
		return nil
	}
	return append([]FieldError(nil), appErr.fields...)
}
```

- [ ] **Step 4: Implement weekly input types, validation, IDs, and fingerprinting**

`validateWeeklyPlan` must emit errors in deterministic document order. `weeklyPlanFingerprint` must JSON-marshal a canonical struct containing name, status, weekdays, titles, and ordered item values, then return lowercase hex SHA-256.

Creation assigns UUIDs to plan, days, and items, sets status `active`, and stores the request fingerprint. Replacement preserves supplied IDs and rejects empty `expected_updated_at`.

Creation accepts only a complete document and always persists `active`. Replacement permits `active`, `draft`, or `archived`, but applies the completeness requirement whenever the requested status is `active`.

- [ ] **Step 5: Prevent legacy activation of incomplete plans**

In `UpdatePlan`, before changing status to active:

```go
if in.Status == "active" {
	weeklyRepo, ok := s.repo.(WeeklyPlanRepository)
	if !ok {
		return p, apperror.Internal("weekly plan repository unavailable")
	}
	complete, err := weeklyRepo.HasCompleteSchedule(c, u, id)
	if err != nil {
		return p, err
	}
	if !complete {
		return p, apperror.InvalidFields(
			"weekly plan is incomplete",
			[]apperror.FieldError{{Path: "days", Message: "至少安排一天且每一天至少包含一个训练项目"}},
		)
	}
}
```

- [ ] **Step 6: Map aggregate RPCs and validation details**

Implement protobuf↔service conversion helpers. For validation errors, attach `google.rpc.BadRequest` details:

```go
st := status.New(codes.InvalidArgument, e.Error())
violations := make([]*errdetails.BadRequest_FieldViolation, 0, len(fields))
for _, field := range fields {
	violations = append(violations, &errdetails.BadRequest_FieldViolation{
		Field:       field.Path,
		Description: field.Message,
	})
}
withDetails, detailErr := st.WithDetails(&errdetails.BadRequest{FieldViolations: violations})
if detailErr != nil {
	return st.Err()
}
return withDetails.Err()
```

Parse `expected_updated_at` with `time.RFC3339Nano`. Map malformed values to `days`-independent invalid argument errors.

- [ ] **Step 7: Verify service and gRPC packages**

Run:

```powershell
go test ./pkg/apperror ./services/plan-service/internal/service ./services/plan-service/internal/grpc -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit Task 4**

```powershell
git add pkg/apperror services/plan-service/internal/service services/plan-service/internal/grpc
git commit -m "feat: validate complete weekly plans"
```

---

### Task 5: Expose Atomic Weekly Plans Through the Gateway

**Files:**
- Modify: `services/api-gateway/internal/mapper/errors.go`
- Modify: `services/api-gateway/internal/mapper/errors_test.go`
- Modify: `services/api-gateway/internal/http/handlers.go`
- Create: `services/api-gateway/internal/http/weekly_plans.go`
- Create: `services/api-gateway/internal/http/weekly_plans_test.go`
- Modify: `services/api-gateway/internal/http/contracts_test.go`

**Interfaces:**
- Produces REST endpoints:
  - `POST /api/v1/weekly-plans`
  - `GET /api/v1/weekly-plans/:plan_id`
  - `PUT /api/v1/weekly-plans/:plan_id`
- Extends mapper response:

```go
type Response struct {
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	RequestID   string            `json:"request_id"`
	FieldErrors map[string]string `json:"field_errors,omitempty"`
}
```

- [ ] **Step 1: Write HTTP contract RED tests**

Use a fake `PlanServiceClient` that captures exactly one RPC. Test:

```go
func TestCreateWeeklyPlanUsesTrustedUserAndHeaderIdempotencyKey(t *testing.T)
func TestReplaceWeeklyPlanForwardsExpectedUpdatedAt(t *testing.T)
func TestWeeklyPlanValidationDetailsBecomeFieldErrors(t *testing.T)
func TestWeeklyPlanBodyOverOneMiBReturns413WithoutCallingGRPC(t *testing.T)
```

The create test must assert that a client-supplied `user_id` is ignored and the JWT subject is used.

- [ ] **Step 2: Run Gateway tests and verify RED**

Run:

```powershell
go test ./services/api-gateway/internal/http ./services/api-gateway/internal/mapper -run 'WeeklyPlan|FieldErrors' -count=1
```

Expected: FAIL because routes and field error extraction are missing.

- [ ] **Step 3: Extract gRPC field violations in the mapper**

Iterate `status.Convert(err).Details()`, find `*errdetails.BadRequest`, and copy each violation into `FieldErrors`. Keep `field_errors` nil for all other errors.

Construct responses with named fields:

```go
return Response{
	Code:        code,
	Message:     message,
	RequestID:   requestID,
	FieldErrors: fieldErrors,
}
```

- [ ] **Step 4: Register weekly routes**

Add:

```go
protected.POST("/weekly-plans", h.createWeeklyPlan)
protected.GET("/weekly-plans/:plan_id", h.getWeeklyPlan)
protected.PUT("/weekly-plans/:plan_id", h.replaceWeeklyPlan)
```

- [ ] **Step 5: Implement one-RPC handlers**

Each handler:

- binds the corresponding protobuf request;
- overwrites `UserId` from trusted context;
- overwrites path `PlanId`;
- takes create idempotency from `Idempotency-Key`;
- uses `grpcContext`;
- sends exactly one plan-service RPC;
- returns 201 for create and 200 for get/replace.

Do not decompose the document into legacy plan/day/item calls.

- [ ] **Step 6: Verify Gateway and checkin compatibility**

Run:

```powershell
go test ./services/api-gateway/internal/http ./services/api-gateway/internal/mapper ./services/api-gateway/internal/clients -count=1
go test ./services/checkin-service/internal/service ./services/checkin-service/cmd -count=1
```

Expected: PASS. Existing `GetWorkoutItem` tests must remain unchanged and passing.

- [ ] **Step 7: Commit Task 5**

```powershell
git add services/api-gateway/internal/http services/api-gateway/internal/mapper
git commit -m "feat: expose weekly plan aggregate API"
```

---

### Task 6: Add Frontend Weekly Contracts, API, and Shared Today Queries

**Files:**
- Modify: `frontend/src/shared/api/contracts.ts`
- Modify: `frontend/src/shared/api/client.ts`
- Modify: `frontend/src/shared/api/client.test.ts`
- Create: `frontend/src/features/plans/weeklyPlanSchema.ts`
- Create: `frontend/src/features/plans/weeklyPlanSchema.test.ts`
- Create: `frontend/src/features/plans/weeklyPlanDraft.ts`
- Create: `frontend/src/features/plans/weeklyPlanDraft.test.ts`
- Modify: `frontend/src/features/plans/api.ts`
- Modify: `frontend/src/features/plans/queries.ts`
- Create: `frontend/src/features/plans/weekly-api.test.ts`
- Create: `frontend/src/features/today-workouts/deriveTodayWorkouts.ts`
- Create: `frontend/src/features/today-workouts/deriveTodayWorkouts.test.ts`
- Create: `frontend/src/features/today-workouts/queries.ts`
- Create: `frontend/src/features/today-workouts/queries.test.tsx`

**Interfaces:**
- Produces:

```ts
export type WeeklyWorkout = {
  id: string;
  plan_id: string;
  weekday: number;
  title: string;
  items: WorkoutItem[];
  created_at: string;
  updated_at: string;
};

export type WeeklyPlanDocument = {
  plan: Plan;
  days: WeeklyWorkout[];
};

export type WeeklyPlanInput = {
  name: string;
  status: Plan["status"];
  days: Array<{
    id?: string;
    weekday: number;
    title: string;
    items: WeeklyItemInput[];
  }>;
};

export type WeeklyItemInput = {
  id?: string;
  name: string;
  sets: number;
  repetitions: number;
  weight: number;
  duration_seconds: number;
};
```

- Produces API:

```ts
createWeeklyPlan(input: WeeklyPlanInput, key: string): Promise<{ weekly_plan: WeeklyPlanDocument }>
getWeeklyPlan(id: string): Promise<{ weekly_plan: WeeklyPlanDocument }>
replaceWeeklyPlan(id: string, expectedUpdatedAt: string, input: WeeklyPlanInput): Promise<{ weekly_plan: WeeklyPlanDocument }>
```

- Produces:

```ts
useWeeklyPlanQuery(planId: string)
useCreateWeeklyPlanMutation()
useReplaceWeeklyPlanMutation(planId: string)
useTodayWorkoutsQuery(date: string)
isoWeekdayFromLocalDate(date: string): 1 | 2 | 3 | 4 | 5 | 6 | 7
```

- [ ] **Step 1: Write schema and pure weekday RED tests**

Test Sunday and Monday explicitly without parsing `YYYY-MM-DD` as UTC:

```ts
expect(isoWeekdayFromLocalDate("2026-08-09")).toBe(7);
expect(isoWeekdayFromLocalDate("2026-08-10")).toBe(1);
```

Test schema errors for duplicate weekdays, empty weekday items, empty item name, negative weight, and all-zero sets/repetitions/duration.

- [ ] **Step 2: Run focused tests and verify RED**

Run:

```powershell
Set-Location frontend
npm run test:run -- src/features/plans/weeklyPlanSchema.test.ts src/features/today-workouts/deriveTodayWorkouts.test.ts
```

Expected: FAIL because files/functions do not exist.

- [ ] **Step 3: Extend error and weekly document contracts**

Change:

```ts
export type ApiErrorBody = {
  code: string;
  message: string;
  request_id: string;
  field_errors?: Record<string, string>;
};
```

Add the weekly types above. Keep legacy `WorkoutDay` and `workout_day_id` during the compatibility period.

- [ ] **Step 4: Implement Zod schema and pure derivation**

`weeklyPlanSchema` must use `superRefine` to emit paths matching the backend, such as:

```ts
ctx.addIssue({
  code: "custom",
  path: ["days", dayIndex, "items"],
  message: "请至少添加一个训练项目"
});
```

`isoWeekdayFromLocalDate` splits the string, constructs `new Date(year, month - 1, day, 12)`, and converts `Date#getDay()` with `day === 0 ? 7 : day`. It rejects dates that do not round-trip to the same local calendar fields. `deriveTodayWorkouts` groups all matching active documents by plan and marks completion from a `Set<workout_item_id>`.

- [ ] **Step 5: Implement aggregate API and query keys**

Use exact keys:

```ts
["weekly-plan", planId]
["today-workouts", date]
["today-checkins", date]
```

`useTodayWorkoutsQuery` must:

1. call existing `listAllPlans`;
2. filter active plans;
3. fetch details with `Promise.all(active.map(plan => getWeeklyPlan(plan.id)))`;
4. fetch every history page for `from=date&to=date`;
5. return groups from `deriveTodayWorkouts`.

- [ ] **Step 6: Test API envelopes, cache invalidation, and all-page history**

MSW tests must assert exact `/api/v1/weekly-plans` request bodies and verify that a completed item on history page 2 is still marked complete.

- [ ] **Step 7: Run frontend contract tests**

Run:

```powershell
npm run test:run -- src/features/plans src/features/today-workouts src/shared/api/client.test.ts
npm run typecheck
```

Expected: PASS.

- [ ] **Step 8: Commit Task 6**

```powershell
git add frontend/src/shared/api frontend/src/features/plans frontend/src/features/today-workouts
git commit -m "feat: add weekly plan frontend contracts"
```

---

### Task 7: Build the Single-Page Weekly Plan Editor

**Files:**
- Modify: `frontend/src/app/App.tsx`
- Modify: `frontend/src/features/plans/PlansPage.tsx`
- Replace: `frontend/src/features/plans/PlanDetailPage.tsx`
- Delete: `frontend/src/features/plans/PlanForm.tsx`
- Delete: `frontend/src/features/plans/PlanEditForm.tsx`
- Delete: `frontend/src/features/plans/WorkoutDayForm.tsx`
- Delete: `frontend/src/features/plans/WorkoutItemForm.tsx`
- Create: `frontend/src/features/plans/WeeklyPlanEditor.tsx`
- Create: `frontend/src/features/plans/WeekdayCard.tsx`
- Create: `frontend/src/features/plans/WorkoutItemEditor.tsx`
- Create: `frontend/src/features/plans/StickySaveBar.tsx`
- Create: `frontend/src/features/plans/UnsavedChangesGuard.tsx`
- Create: `frontend/src/features/plans/WeeklyPlanEditor.module.css`
- Modify: `frontend/src/shared/ui/Button.tsx`
- Modify: `frontend/src/features/plans/plans.test.tsx`
- Create: `frontend/src/features/plans/weekly-editor.test.tsx`
- Modify: `frontend/src/app/App.test.tsx`

**Interfaces:**
- `/plans/new` renders creation mode.
- `/plans/:planId` renders edit mode.
- `WeeklyPlanEditor` accepts:

```ts
type WeeklyPlanEditorProps =
  | { mode: "create" }
  | { mode: "edit"; planId: string };
```

- `Button` becomes:

```ts
type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "danger";
};
```

- [ ] **Step 1: Write editor RED tests**

Cover:

```ts
it("creates name, weekdays, and items with one aggregate request")
it("prevents save when a weekday has no items and focuses its add-item control")
it("maps backend field_errors onto nested controls without losing the draft")
it("shows a 409 refresh warning without overwriting local edits")
it("allows one expanded weekday on mobile and multiple on desktop")
it("warns before route navigation when the draft is dirty")
```

The successful test must assert one POST to `/api/v1/weekly-plans` and zero requests to legacy `/plans/:id/days` or `/workout-days/:id/items`.

- [ ] **Step 2: Run editor tests and verify RED**

Run:

```powershell
npm run test:run -- src/features/plans/weekly-editor.test.tsx src/app/app.test.tsx
```

Expected: FAIL because editor and route do not exist.

- [ ] **Step 3: Upgrade the router for reliable navigation blocking**

Replace `BrowserRouter`/`Routes` construction with `createBrowserRouter` and `RouterProvider`, preserving the exact existing public and protected routes while adding `/plans/new`.

Keep `QueryClientProvider` and `SessionProvider` above `RouterProvider`. Verify login redirect, protected routes, and logout tests before continuing.

- [ ] **Step 4: Extend Button without losing existing styles**

Spread standard button props and merge caller style:

```tsx
export function Button({ variant = "primary", style, disabled, ...props }: ButtonProps) {
  return (
    <button
      {...props}
      disabled={disabled}
      style={{
        ...baseStyle,
        ...variantStyles[variant],
        opacity: disabled ? 0.6 : 1,
        ...style
      }}
    />
  );
}
```

- [ ] **Step 5: Implement editor form composition**

Use one `useForm<WeeklyPlanInput>` and a top-level `useFieldArray({ name: "days" })`. Each `WeekdayCard` uses a nested `useFieldArray({ name: \`days.${index}.items\` })`.

Rules:

- generated local IDs are UI keys only and are removed before POST;
- existing server IDs are retained on PUT;
- mobile uses `matchMedia("(max-width: 767px)")` and one expanded ID;
- desktop tracks a set of expanded IDs;
- server field paths call `setError(path, { message })`;
- unknown paths render in a page-level `Feedback`.

- [ ] **Step 6: Implement sticky save and unsaved changes guard**

`StickySaveBar` displays day/item counts and includes bottom spacing for the mobile navigation safe area.

`UnsavedChangesGuard` uses `useBlocker(isDirty)` and `useBeforeUnload`:

```tsx
const blocker = useBlocker(isDirty);
useBeforeUnload(
  useCallback((event) => {
    if (!isDirty) return;
    event.preventDefault();
  }, [isDirty])
);
```

Render a confirmation dialog with “继续编辑” and “放弃修改并离开”; do not use an untestable implicit navigation.

- [ ] **Step 7: Replace old creation and detail flows**

`PlansPage` “新建计划” navigates to `/plans/new`; remove inline `PlanForm`.

`PlanDetailPage` becomes a small route adapter:

```tsx
export function PlanDetailPage() {
  const { planId = "" } = useParams<{ planId: string }>();
  return <WeeklyPlanEditor mode="edit" planId={planId} />;
}
```

Delete old date/item forms only after all imports and tests no longer reference them.

- [ ] **Step 8: Verify editor, routing, accessibility attributes, and build**

Run:

```powershell
npm run test:run -- src/app src/features/plans
npm run typecheck
npm run lint
npm run build
```

Expected: PASS with no React act warnings or unhandled MSW requests.

- [ ] **Step 9: Commit Task 7**

```powershell
git add frontend/src/app frontend/src/features/plans frontend/src/shared/ui/Button.tsx
git commit -m "feat: add single-page weekly plan editor"
```

---

### Task 8: Replace Dropdown Check-In and Share Today Workouts with Dashboard

**Files:**
- Create: `frontend/src/features/today-workouts/TodayWorkoutGroup.tsx`
- Create: `frontend/src/features/today-workouts/CheckinItemCard.tsx`
- Create: `frontend/src/features/today-workouts/AlternateWeekdayPicker.tsx`
- Create: `frontend/src/features/today-workouts/TodayWorkouts.module.css`
- Replace: `frontend/src/features/checkins/CheckinPage.tsx`
- Modify: `frontend/src/features/checkins/checkin.test.tsx`
- Modify: `frontend/src/features/checkins/api.ts`
- Modify: `frontend/src/features/dashboard/DashboardPage.tsx`
- Modify: `frontend/src/features/dashboard/dashboard.test.tsx`
- Modify: `frontend/src/features/dashboard/DashboardPage.module.css`

**Interfaces:**
- `CheckinItemCard` accepts:

```ts
type CheckinItemCardProps = {
  item: WorkoutItem;
  date: string;
  completed: boolean;
};
```

- `AlternateWeekdayPicker` accepts all active weekly documents and emits a selected plan/day pair without mutating the plan.
- Dashboard and Checkin both consume `useTodayWorkoutsQuery(todayLocalDate())`.

- [ ] **Step 1: Rewrite check-in and Dashboard tests for the approved flow**

Add tests:

```ts
it("shows every active plan's matching weekday without selectors")
it("completes one item without disabling other item cards")
it("reuses an item's idempotency key after 503 and marks a 409 as completed")
it("chooses another weekday but submits today's local date")
it("shows the same today groups on Dashboard and Checkin")
```

Assert that labels “训练计划”, “训练日”, and “训练项目” are absent from Checkin.

- [ ] **Step 2: Run focused tests and verify RED**

Run:

```powershell
npm run test:run -- src/features/checkins/checkin.test.tsx src/features/dashboard/dashboard.test.tsx src/features/today-workouts
```

Expected: FAIL because current pages still use date matching and selectors.

- [ ] **Step 3: Implement per-item idempotent mutations**

Each item card stores `{ identity, key }`, where identity is:

```ts
JSON.stringify({ workout_item_id: item.id, date, note: "" })
```

On 503/network failure, retry with the same key. After success, clear the attempt. On 409, invalidate today history and render completed state.

Invalidate:

```ts
["today-checkins", date]
["today-workouts", date]
["history"]
["streak"]
["statistics"]
```

- [ ] **Step 4: Implement grouped today and alternate weekday UI**

The default view renders all groups returned by `useTodayWorkoutsQuery`.

When no groups exist, render “今天休息” and the alternate picker. The picker uses plan cards and weekday cards; selecting a day renders the same `CheckinItemCard` list with `date` still equal to today.

- [ ] **Step 5: Replace Dashboard’s first-plan/date logic**

Remove `mainPlan`, `useWorkoutDaysQuery`, `todayDay`, and `useWorkoutItemsQuery`. Render shared today groups and preserve the existing statistics, streak, body data, and recent-history cards.

- [ ] **Step 6: Verify frontend tests and build**

Run:

```powershell
npm run test:run -- src/features/today-workouts src/features/checkins src/features/dashboard
npm run test:run
npm run typecheck
npm run lint
npm run build
```

Expected: PASS.

- [ ] **Step 7: Commit Task 8**

```powershell
git add frontend/src/features/today-workouts frontend/src/features/checkins frontend/src/features/dashboard
git commit -m "feat: simplify today workout check-ins"
```

---

### Task 9: Add Deployment Guardrails, End-to-End Coverage, and Documentation

**Files:**
- Modify: `deploy/deployment_contract_test.go`
- Modify: `deploy/k8s/base/services.yaml`
- Modify: `README.md`
- Modify: `tests/e2e/fitness_flow_test.go`
- Modify: `frontend/e2e/fitness-flow.spec.ts`
- Modify: `frontend/e2e/accessibility.spec.ts`
- Modify: `frontend/e2e/layout.spec.ts`

**Interfaces:**
- Compose and Kubernetes must not start old and new plan-service writers concurrently during the migration.
- Browser E2E covers desktop and Pixel 5.
- Go E2E uses the aggregate endpoint while retaining one legacy compatibility assertion.

- [ ] **Step 1: Write deployment contract RED tests**

Add static assertions that:

- plan-service deployment strategy is `Recreate`;
- plan-service remains one replica during this migration;
- documentation names migration version `2026081301`;
- Compose rebuild/recreate instructions stop plan-service before starting the new image.

Example Kubernetes assertions:

```go
if !strings.Contains(servicesYAML, "strategy:\n      type: Recreate") {
	t.Fatal("plan-service must use Recreate during weekday migration")
}
if !strings.Contains(servicesYAML, "replicas: 1") {
	t.Fatal("plan-service migration requires one writer")
}
```

- [ ] **Step 2: Run deployment test and verify RED**

Run:

```powershell
go test ./deploy -run WeeklyPlanMigration -count=1
```

Expected: FAIL until deployment manifests and docs contain the guardrails.

- [ ] **Step 3: Update deployment and operating documentation**

Document the exact order:

```powershell
docker compose -f deploy/docker-compose.yml stop plan-service
docker compose -f deploy/docker-compose.yml build plan-service api-gateway frontend
docker compose -f deploy/docker-compose.yml up -d plan-service
docker compose -f deploy/docker-compose.yml up -d api-gateway checkin-service frontend
```

Explain that `WeeklyMigrationConflictError` must be resolved before retrying and that no data is changed when the migration transaction fails.

For Kubernetes, set plan-service `strategy.type: Recreate` and keep one replica for this migration release.

- [ ] **Step 4: Rewrite Go E2E for atomic weekly creation**

The E2E must:

1. register;
2. POST one complete weekly plan;
3. GET and compare every weekday/item ID;
4. use an item ID with existing checkin endpoint;
5. verify history;
6. reuse the same idempotency key with changed content and expect 409;
7. verify legacy `POST /plans` returns a draft plan.

- [ ] **Step 5: Rewrite browser E2E for the visible workflow**

`fitness-flow.spec.ts` must create multiple weekdays/items in one editor, save once, reopen, navigate to Checkin, complete the matching item directly, and verify history/Dashboard/statistics.

Run the same spec in both configured projects. The test must assert no three selector labels exist.

- [ ] **Step 6: Expand accessibility and layout E2E**

Add Axe checks for:

- editor with one weekday expanded;
- nested validation errors;
- today workout cards;
- alternate weekday picker.

Add layout assertions:

- 44×44 controls;
- only one mobile weekday expanded;
- sticky save bar does not overlap bottom navigation or the last item field;
- desktop allows multiple expanded weekdays.

- [ ] **Step 7: Run the complete verification matrix**

Run:

```powershell
go test ./... -count=1
go vet ./...

$env:TEST_DATABASE_ADMIN_DSN='postgres://fitness:postgres-local-only@127.0.0.1:5432/fitness?sslmode=disable'
go test -tags=integration ./services/plan-service/internal/repository -count=1

Set-Location frontend
npm run test:run
npm run typecheck
npm run lint
npm run build

Set-Location ..
docker compose -f deploy/docker-compose.yml up -d --build
$env:BASE_URL='http://127.0.0.1:8088'
go test -tags=e2e ./tests/e2e -count=1

Set-Location frontend
$env:PLAYWRIGHT_BASE_URL='http://127.0.0.1:8088'
npm run e2e
```

Expected:

- all Go tests and vet pass;
- migration integration tests pass;
- all Vitest suites, typecheck, lint, and build pass;
- Go E2E passes;
- desktop/mobile Playwright, Axe, and layout suites pass.

- [ ] **Step 8: Commit Task 9**

```powershell
git add deploy README.md tests/e2e frontend/e2e
git commit -m "test: verify weekly fitness workflow"
```

---

## Execution Notes

- Execute tasks strictly in order because generated proto types, migration schema, aggregate repository, API, and frontend depend on preceding interfaces.
- Every implementation task begins with a failing test and records the expected RED failure before production changes.
- Do not manually edit protobuf-generated Go files.
- Do not delete the legacy `GetWorkoutItem` RPC or renumber its fields.
- Do not use full-delete/recreate for retained workout items.
- If migration detects duplicate weekdays, stop implementation verification and resolve the data conflict explicitly; do not bypass the preflight.
- Before each commit, review `git diff` and exclude `.superpowers/`, generated test artifacts, credentials, and local database files.
