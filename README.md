# go-audit

Go library to track changes on your process — who did it, what changed, from what to what. Port of [laravel-audit](https://github.com/iqbalatma/laravel-audit), redesigned to be framework/ORM agnostic: this package has no opinion on your HTTP framework or your database layer.

## Install

```console
go get github.com/iqbalatma/go-audit
```

## How it works

- `Header` — one row per action: actor, request info, entry object, action/message, tag, additional.
- `Trail` — one row per changed record: object table/id, before, after.
- You implement `Storer` to decide where/how these get saved. `go-audit` never touches your DB directly.

## Quick usage

Attach actor & request info to `context.Context` once, in your HTTP middleware — how you do that depends on your framework:

### net/http or chi

`Middleware` is ready to use as-is, no glue code needed:

```go
r := chi.NewRouter() // or any net/http-compatible router
r.Use(audit.Middleware)

// actor still needs your own auth middleware, chained after:
r.Use(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if user := getAuthUser(r); user != nil {
            ctx := audit.WithActor(r.Context(), audit.Actor{Table: "users", ID: user.ID, Name: user.Name})
            r = r.WithContext(ctx)
        }
        next.ServeHTTP(w, r)
    })
})
```

### gin

gin's `*gin.Context` isn't an `http.Handler`, so `Middleware` doesn't plug in directly — but `RequestInfoFromHTTP` does the same IP/method/endpoint parsing, called from your own 3-line middleware:

```go
func AuditMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx := audit.WithRequestInfo(c.Request.Context(), audit.RequestInfoFromHTTP(c.Request))
        if user := getAuthUser(c); user != nil {
            ctx = audit.WithActor(ctx, audit.Actor{Table: "users", ID: user.ID, Name: user.Name})
        }
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}

r := gin.Default()
r.Use(AuditMiddleware())
```

### echo

Same pattern, echo just exposes the request via `c.Request()` instead of a struct field:

```go
func AuditMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        ctx := audit.WithRequestInfo(c.Request().Context(), audit.RequestInfoFromHTTP(c.Request()))
        if user := getAuthUser(c); user != nil {
            ctx = audit.WithActor(ctx, audit.Actor{Table: "users", ID: user.ID, Name: user.Name})
        }
        c.SetRequest(c.Request().WithContext(ctx))
        return next(c)
    }
}

e := echo.New()
e.Use(AuditMiddleware)
```

None of this requires gin/echo as a dependency of `go-audit` — `RequestInfoFromHTTP` takes a plain `*http.Request`, which both frameworks expose.

Then in your service layer, same regardless of framework:

```go
// create — before is nil, full "after" is stored
audit.Init(ctx, "CREATE_PRODUCT", "create product via ProductService.CreateProduct").
    SetEntryObject("products", product.ID).
    AddSingleTrail("products", product.ID, nil, toMap(product), nil, nil).
    Execute(ctx)

// update — only changed fields are stored, with a per-trail tag
audit.Init(ctx, "UPDATE_ROLE", "update role via RoleService.UpdateRole").
    SetEntryObject("roles", role.ID).
    AddSingleTrail("roles", role.ID, beforeMap, toMap(role), map[string]any{"level": "important"}, nil).
    Execute(ctx)

// relation sync — whole before/after set is stored, no diffing
audit.Init(ctx, "SYNC_ROLE_PERMISSIONS", "sync role permissions").
    SetEntryObject("roles", roleID).
    AddRelationalTrail("permissions", before, after, nil, nil).
    Execute(ctx)

// override the app name for one audit instead of using Configure's default
audit.Init(ctx, "CREATE_PRODUCT", "create product").
    SetAppName("Admin Panel").
    SetEntryObject("products", product.ID).
    AddSingleTrail("products", product.ID, nil, toMap(product), nil, nil).
    Execute(ctx)
```

`Execute(ctx)` runs the save in a goroutine and does not block. Call it *after* your own transaction has committed — there is no "after commit" hook here, unlike Laravel's queue.

## GORM `Storer` implementation

Copy this into your project (e.g. `internal/audit/gorm_storer.go`). It's not part of this module on purpose — adding a GORM dependency to `go-audit` itself would force it on consumers who don't use GORM.

### Models

```go
package audit

import "time"

type AuditModel struct {
    ID               string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Action           string
    Message          string
    AppName          string
    ActorTable       string
    ActorID          string
    ActorName        string
    ActorEmail       string
    ActorPhone       string
    IPAddress        string
    Method           string
    Endpoint         string
    UserAgent        string
    EntryObjectTable string
    EntryObjectID    string
    Tag              string // JSON
    Additional       string // JSON
    CreatedAt        time.Time
    Trails           []AuditTrailModel `gorm:"foreignKey:AuditID"`
}

func (AuditModel) TableName() string { return "audits" }

type AuditTrailModel struct {
    ID          string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    AuditID     string
    ObjectTable string
    ObjectID    string
    Before      string // JSON
    After       string // JSON
    Tag         string // JSON
    Additional  string // JSON
    CreatedAt   time.Time
}

func (AuditTrailModel) TableName() string { return "audit_trails" }
```

### Migration (raw SQL — adapt to your migration tool)

```sql
CREATE TABLE audits (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    action              text,
    message             text,
    app_name            text,
    actor_table         text,
    actor_id            text,
    actor_name          text,
    actor_email         text,
    actor_phone         text,
    ip_address          text,
    method              text,
    endpoint            text,
    user_agent          text,
    entry_object_table  text,
    entry_object_id     text,
    tag                 jsonb,
    additional          jsonb,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_trails (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    audit_id     uuid NOT NULL REFERENCES audits(id),
    object_table text,
    object_id    text,
    before       jsonb,
    after        jsonb,
    tag          jsonb,
    additional   jsonb,
    created_at   timestamptz NOT NULL DEFAULT now()
);
```

### Storer

```go
package audit

import (
    "context"
    "encoding/json"
    "fmt"

    goaudit "github.com/iqbalatma/go-audit"
    "gorm.io/gorm"
)

type GormStorer struct {
    db *gorm.DB
}

func NewGormStorer(db *gorm.DB) *GormStorer {
    return &GormStorer{db: db}
}

func (s *GormStorer) Save(ctx context.Context, h goaudit.Header, trails []goaudit.Trail) error {
    return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        tag, _ := json.Marshal(h.Tag)
        additional, _ := json.Marshal(h.Additional)

        row := AuditModel{
            Action:           h.Action,
            Message:          h.Message,
            AppName:          h.AppName,
            ActorTable:       h.Actor.Table,
            ActorID:          idString(h.Actor.ID),
            ActorName:        h.Actor.Name,
            ActorEmail:       h.Actor.Email,
            ActorPhone:       h.Actor.Phone,
            IPAddress:        h.Request.IPAddress,
            Method:           h.Request.Method,
            Endpoint:         h.Request.Endpoint,
            UserAgent:        h.Request.UserAgent,
            EntryObjectTable: h.EntryObjectTable,
            EntryObjectID:    idString(h.EntryObjectID),
            Tag:              string(tag),
            Additional:       string(additional),
        }
        if err := tx.Create(&row).Error; err != nil {
            return err
        }

        for _, t := range trails {
            before, _ := json.Marshal(t.Before)
            after, _ := json.Marshal(t.After)
            tag, _ := json.Marshal(t.Tag)
            additional, _ := json.Marshal(t.Additional)

            trailRow := AuditTrailModel{
                AuditID:     row.ID,
                ObjectTable: t.ObjectTable,
                ObjectID:    idString(t.ObjectID),
                Before:      string(before),
                After:       string(after),
                Tag:         string(tag),
                Additional:  string(additional),
            }
            if err := tx.Create(&trailRow).Error; err != nil {
                return err
            }
        }
        return nil
    })
}

// idString avoids fmt.Sprint(nil) turning into the literal string "<nil>"
// when an actor or entry object has no id set.
func idString(id any) string {
    if id == nil {
        return ""
    }
    return fmt.Sprint(id)
}
```

### Wire it up at startup

```go
db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})
audit.Configure(auditpkg.NewGormStorer(db), "MyApp")
```

## Known limitations (deliberate, see code comments for upgrade path)

- Diff skips nested array/object fields — only scalar field changes are tracked per trail.
- No automatic request-body capture (unlike laravel-audit's `user_request` column) — pass what you need via `Additional`.
