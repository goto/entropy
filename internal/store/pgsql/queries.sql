-- =================== Modules ===================

-- name: GetModuleByURN :one
SELECT *
FROM modules
WHERE urn = $1;

-- name: ListAllModulesForProject :many
SELECT *
FROM modules
WHERE project = $1;

-- name: InsertModule :exec
INSERT INTO modules (urn, project, name, configs)
VALUES ($1, $2, $3, $4);

-- name: UpdateModule :exec
UPDATE modules
SET configs    = $2,
    updated_at = current_timestamp
WHERE urn = $1;

-- name: DeleteModule :exec
DELETE
FROM modules
WHERE urn = $1;

-- =================== Resources ===================

-- name: GetResourceByURN :one
SELECT r.*,
       array_agg(rt.tag)::text[] AS tags,
        jsonb_object_agg(COALESCE(rd.dependency_key, ''), d.urn) AS dependencies
FROM resources r
         LEFT JOIN resource_tags rt ON r.id = rt.resource_id
         LEFT JOIN resource_dependencies rd ON r.id = rd.resource_id
         LEFT JOIN resources d ON rd.depends_on = d.id
WHERE r.urn = $1
GROUP BY r.id;

-- name: GetResourceDependencies :one
SELECT (CASE
            WHEN COUNT(rd.dependency_key) > 0 THEN
                json_object_agg(rd.dependency_key, d.urn)
            ELSE
                '{}'::json
    END) AS dependencies
FROM resources r
         LEFT JOIN resource_dependencies rd ON r.id = rd.resource_id
         LEFT JOIN resources d ON rd.depends_on = d.id
WHERE r.urn = $1
GROUP BY r.id;

-- name: ListResourceURNsByFilter :many
SELECT r.*,
       array_agg(rt.tag)::text[] AS tags,
       jsonb_object_agg(COALESCE(rd.dependency_key, ''), d.urn) AS dependencies
FROM resources r
         LEFT JOIN resource_dependencies rd ON r.id = rd.resource_id
         LEFT JOIN resources d ON rd.depends_on = d.id
         LEFT JOIN resource_tags rt ON r.id = rt.resource_id
WHERE (sqlc.narg('project')::text IS NULL OR r.project = sqlc.narg('project'))
  AND (sqlc.narg('kind')::text IS NULL OR r.kind = sqlc.narg('kind'))
GROUP BY r.id;

-- name: DeleteResourceDependenciesByURN :exec
DELETE
FROM resource_dependencies
WHERE resource_id = (SELECT id FROM resources WHERE urn = $1);

-- name: DeleteResourceTagsByURN :exec
DELETE
FROM resource_tags
WHERE resource_id = (SELECT id FROM resources WHERE urn = $1);

-- name: DeleteResourceByURN :exec
DELETE
FROM resources
WHERE urn = $1;

-- name: InsertResource :one
INSERT INTO resources ("urn", "kind", "project", "name", "created_at", "updated_at", "created_by", "updated_by",
                       "spec_configs", "state_status", "state_output", "state_module_data",
                       "state_next_sync", "state_sync_result")
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id;

-- name: InsertResourceTags :copyfrom
INSERT INTO resource_tags (resource_id, tag)
VALUES ($1, $2);

-- name: InsertResourceDependency :exec
INSERT INTO resource_dependencies (resource_id, dependency_key, depends_on)
VALUES ($1, $2, (SELECT id FROM resources WHERE urn = $3));

-- name: UpdateResource :one
UPDATE resources
SET updated_at        = current_timestamp,
    updated_by        = $2,
    spec_configs      = $3,
    state_status      = $4,
    state_output      = $5,
    state_module_data = $6,
    state_next_sync   = $7,
    state_sync_result = $8
WHERE urn = $1
RETURNING id;

-- =================== Revisions ===================

-- name: ListResourceRevisions :many
SELECT rev.*, array_agg(distinct rt.tag)::text[] AS tags
FROM resources r
         JOIN revisions rev ON r.id = rev.resource_id
         JOIN revision_tags rt ON rev.id = rt.revision_id
WHERE r.urn = $1
GROUP BY rev.id;

-- name: InsertRevision :exec
INSERT INTO revisions ("resource_id", "reason", "spec_configs", "created_by")
VALUES ($1, $2, $3, $4);