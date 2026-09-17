package audit

import (
	"context"
	"log"
	"maps"
)

// Trail is one changed record captured within an audit.
type Trail struct {
	ObjectTable string
	ObjectID    any
	Before      any
	After       any
	Tag         map[string]any
	Additional  map[string]any
}

// Header is the top-level audit record.
type Header struct {
	Action           string
	Message          string
	AppName          string
	Actor            Actor
	Request          RequestInfo
	EntryObjectTable string
	EntryObjectID    any
	Tag              map[string]any
	Additional       map[string]any
}

// Storer persists a completed audit. Implement it against whatever DB/ORM you use
// (raw database/sql, GORM, sqlx, ...) — this package has no opinion on storage.
type Storer interface {
	Save(ctx context.Context, header Header, trails []Trail) error
}

var (
	storer  Storer
	appName string
)

// Configure sets the storage backend once at startup.
func Configure(s Storer, app string) {
	storer = s
	appName = app
}

// Audit builds one audit record via chained calls, then Execute persists it.
type Audit struct {
	header Header
	trails []Trail
}

// Init starts a new audit, pulling actor/request info already attached to ctx
// (see WithActor / WithRequestInfo, typically set by your HTTP middleware).
func Init(ctx context.Context, action, message string) *Audit {
	return &Audit{
		header: Header{
			Action:  action,
			Message: message,
			AppName: appName,
			Actor:   actorFromContext(ctx),
			Request: requestInfoFromContext(ctx),
		},
	}
}

func (a *Audit) SetEntryObject(table string, id any) *Audit {
	a.header.EntryObjectTable = table
	a.header.EntryObjectID = id
	return a
}

func (a *Audit) SetActor(actor Actor) *Audit {
	a.header.Actor = actor
	return a
}

// SetAppName overrides the default app name set via Configure, for this audit only.
func (a *Audit) SetAppName(name string) *Audit {
	a.header.AppName = name
	return a
}

// SetTag merges into any tags already set — safe to call more than once.
func (a *Audit) SetTag(tag map[string]any) *Audit {
	if a.header.Tag == nil {
		a.header.Tag = map[string]any{}
	}
	maps.Copy(a.header.Tag, tag)
	return a
}

// SetAdditional merges into any additional data already set — safe to call more than once.
func (a *Audit) SetAdditional(additional map[string]any) *Audit {
	if a.header.Additional == nil {
		a.header.Additional = map[string]any{}
	}
	maps.Copy(a.header.Additional, additional)
	return a
}

// AddSingleTrail diffs before/after and appends only the changed fields.
// Pass nil for before on create, nil for after on delete — the full value is
// stored as-is in that case, no trail added when there is no actual change.
// tag/additional are optional, pass nil when not needed.
func (a *Audit) AddSingleTrail(table string, id any, before, after any, tag, additional map[string]any) *Audit {
	if before == nil || after == nil {
		a.trails = append(a.trails, Trail{ObjectTable: table, ObjectID: id, Before: before, After: after, Tag: tag, Additional: additional})
		return a
	}

	diffBefore, diffAfter := diffMaps(toMap(before), toMap(after))
	if len(diffBefore) == 0 && len(diffAfter) == 0 {
		return a
	}
	a.trails = append(a.trails, Trail{ObjectTable: table, ObjectID: id, Before: diffBefore, After: diffAfter, Tag: tag, Additional: additional})
	return a
}

// AddRelationalTrail stores before/after as-is, no diffing — for relation
// sync/attach/detach where the whole set matters, not per-field changes.
// tag/additional are optional, pass nil when not needed.
func (a *Audit) AddRelationalTrail(table string, before, after any, tag, additional map[string]any) *Audit {
	a.trails = append(a.trails, Trail{ObjectTable: table, Before: before, After: after, Tag: tag, Additional: additional})
	return a
}

// Execute persists the audit asynchronously. Call it after your own
// transaction has committed — this package has no "after commit" hook.
func (a *Audit) Execute(ctx context.Context) {
	if storer == nil || len(a.trails) == 0 {
		return
	}
	header, trails := a.header, a.trails
	go func() {
		if err := storer.Save(context.WithoutCancel(ctx), header, trails); err != nil {
			log.Printf("audit: save failed: %v", err)
		}
	}()
}
