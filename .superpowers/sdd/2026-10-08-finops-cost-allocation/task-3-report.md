# Task 3 report — freeze allocation at admission

## Implemented

- `ResolveTenant` now loads API-key override, service-account override, and project default under composite workspace/project predicates. All configured candidates are validated before applying API-key > service-account > project > explicit unallocated precedence. Archived cost centers/tags and cross-tenant or project-mismatched references fail closed.
- `TenantContext`, `TenantAdmissionSnapshot`, and `VideoPendingTenantSnapshot` retain defensive allocation copies. Async video pending billing persists the allocation captured at create admission.
- `BudgetService.Reserve` and `Admit` normalize and defensively copy allocation snapshots into `BudgetAttribution`.
- `budgetRepository.Reserve` inserts `budget_reservation_allocation_snapshots` in the reservation transaction. Same request/key retries load and compare the immutable snapshot; configuration drift returns `ErrBudgetReservationConflict` and never overwrites the original.
- Added a focused service test covering allocation copy isolation.

## Validation

Passed:

```text
go test ./internal/service -run 'TestBudgetServiceFreezesAdmissionAllocation|TestBudget' -count=1
go test ./internal/repository -run 'TestBudget' -count=1
go test ./internal/service ./internal/repository ./internal/handler -run '^$'
git diff --check
```

Migration files were not modified by this task.

## Boundary

The full PostgreSQL integration suite and live provider checks were not run in this focused task.
