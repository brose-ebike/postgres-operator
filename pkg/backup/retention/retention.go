/*
Copyright 2026 Yamaha Motor eBike Systems GmbH.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package retention implements the backup retention/salvage algorithm,
// ported from cloud-pgdumper's cleaner.py::_compute_outdated_dumps. Unlike
// cloud-pgdumper, this operates on structured FinishedAt timestamps rather
// than parsing timestamps out of filenames, and is applied per-database by
// the caller (see pkg/backup/worker) rather than globally across every
// backup a policy has ever produced.
package retention

import (
	"sort"
	"time"
)

// Entry is the minimal information the salvage algorithm needs about one
// stored backup. Name is an opaque identifier (the PgBackupInstance name);
// FinishedAt is the authoritative timestamp.
type Entry struct {
	Name       string
	FinishedAt time.Time
}

// Policy mirrors api/v1.PgBackupRetention in plain Go types.
type Policy struct {
	MinCount *int32
	MaxAge   *time.Duration
}

// ComputeOutdated returns the subset of entries that should be deleted.
//
// Semantics:
//  1. If MinCount is set and len(entries) <= *MinCount: delete nothing.
//  2. If MaxAge is unset: pure count-based - keep newest *MinCount (if set),
//     delete the rest; if MinCount is also unset, keep everything.
//  3. If MaxAge is set: split into keep (age < MaxAge) / outdated (age >=
//     MaxAge). If MinCount is set and len(keep) < *MinCount, salvage the
//     newest (*MinCount - len(keep)) entries out of outdated back into
//     keep; whatever remains in outdated is deleted.
//
// entries need not be pre-sorted; ComputeOutdated sorts a copy newest-first.
func ComputeOutdated(entries []Entry, policy Policy, now time.Time) []Entry {
	if len(entries) == 0 {
		return nil
	}

	if policy.MinCount != nil && int32(len(entries)) <= *policy.MinCount {
		return nil
	}

	sorted := make([]Entry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FinishedAt.After(sorted[j].FinishedAt)
	})

	if policy.MaxAge == nil {
		if policy.MinCount == nil {
			return nil
		}
		keepCount := int(*policy.MinCount)
		if keepCount > len(sorted) {
			keepCount = len(sorted)
		}
		return append([]Entry{}, sorted[keepCount:]...)
	}

	cutoff := now.Add(-*policy.MaxAge)
	var keep, outdated []Entry
	for _, e := range sorted {
		if !e.FinishedAt.Before(cutoff) {
			keep = append(keep, e)
		} else {
			outdated = append(outdated, e)
		}
	}

	if policy.MinCount == nil {
		return outdated
	}

	needed := int(*policy.MinCount) - len(keep)
	if needed <= 0 {
		return outdated
	}
	if needed > len(outdated) {
		needed = len(outdated)
	}
	// outdated is already sorted newest-first (it's a stable-ordered subset
	// of sorted), so the first `needed` entries are the ones to salvage.
	return append([]Entry{}, outdated[needed:]...)
}
