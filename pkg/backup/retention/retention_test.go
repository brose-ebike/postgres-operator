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

package retention

import (
	"sort"
	"strconv"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// daysAgo builds an Entry named after how many days before fixedNow it was
// finished, mirroring cloud-pgdumper's test_cleaner_outdated_logic.py's
// make_name helper.
func daysAgo(days int) Entry {
	return Entry{
		Name:       "day-" + strconv.Itoa(days),
		FinishedAt: fixedNow.Add(-time.Duration(days) * 24 * time.Hour),
	}
}

func names(entries []Entry) []string {
	result := make([]string, len(entries))
	for i, e := range entries {
		result[i] = e.Name
	}
	sort.Strings(result)
	return result
}

func containsName(entries []Entry, name string) bool {
	for _, e := range entries {
		if e.Name == name {
			return true
		}
	}
	return false
}

func ptrInt32(v int32) *int32 { return &v }

func ptrDuration(d time.Duration) *time.Duration { return &d }

func TestComputeOutdated_DaysAndCountSalvageNotNeeded(t *testing.T) {
	// Ported from cloud-pgdumper's test_compute_outdated_with_days_and_count:
	// maxAge=30d, minCount=2, ages [0,5,40,41] -> the two newest (0,5)
	// already satisfy minCount via the age-based keep set, so only the two
	// genuinely old entries (40,41) are outdated - no salvage needed.
	entries := []Entry{daysAgo(0), daysAgo(5), daysAgo(40), daysAgo(41)}
	policy := Policy{MinCount: ptrInt32(2), MaxAge: ptrDuration(30 * 24 * time.Hour)}

	outdated := ComputeOutdated(entries, policy, fixedNow)

	if !containsName(outdated, "day-40") || !containsName(outdated, "day-41") {
		t.Fatalf("expected day-40 and day-41 outdated, got %v", names(outdated))
	}
	if len(outdated) != 2 {
		t.Fatalf("expected exactly 2 outdated entries, got %d: %v", len(outdated), names(outdated))
	}
}

func TestComputeOutdated_CountOnly(t *testing.T) {
	// Ported from cloud-pgdumper's test_compute_outdated_count_only:
	// minCount=2 only (no maxAge), ages [0..5] (6 entries) -> keeps the 2
	// newest, deletes the remaining 4.
	entries := []Entry{daysAgo(0), daysAgo(1), daysAgo(2), daysAgo(3), daysAgo(4), daysAgo(5)}
	policy := Policy{MinCount: ptrInt32(2)}

	outdated := ComputeOutdated(entries, policy, fixedNow)

	if len(outdated) != 4 {
		t.Fatalf("expected exactly 4 outdated entries, got %d: %v", len(outdated), names(outdated))
	}
	if containsName(outdated, "day-0") || containsName(outdated, "day-1") {
		t.Fatalf("day-0 and day-1 must be kept, got outdated=%v", names(outdated))
	}
}

func TestComputeOutdated_SalvageAcrossMaxAge(t *testing.T) {
	// New case added during review: minCount=3, maxAge=30d, ages
	// [0,35,36,37] -> keep (age<30d) = {day-0} (len 1 < 3) so salvage the
	// newest 2 of outdated {35,36,37} -> salvage {35,36}; only day-37 is
	// actually deleted.
	entries := []Entry{daysAgo(0), daysAgo(35), daysAgo(36), daysAgo(37)}
	policy := Policy{MinCount: ptrInt32(3), MaxAge: ptrDuration(30 * 24 * time.Hour)}

	outdated := ComputeOutdated(entries, policy, fixedNow)

	if len(outdated) != 1 || !containsName(outdated, "day-37") {
		t.Fatalf("expected only day-37 outdated, got %v", names(outdated))
	}
}

func TestComputeOutdated_EarlyReturnBelowMinCount(t *testing.T) {
	entries := []Entry{daysAgo(0), daysAgo(100)}
	policy := Policy{MinCount: ptrInt32(5), MaxAge: ptrDuration(10 * 24 * time.Hour)}

	outdated := ComputeOutdated(entries, policy, fixedNow)

	if len(outdated) != 0 {
		t.Fatalf("expected nothing deleted when count <= minCount, got %v", names(outdated))
	}
}

func TestComputeOutdated_NoRulesKeepsEverything(t *testing.T) {
	entries := []Entry{daysAgo(0), daysAgo(1000)}
	policy := Policy{}

	outdated := ComputeOutdated(entries, policy, fixedNow)

	if len(outdated) != 0 {
		t.Fatalf("expected nothing deleted with no retention rules, got %v", names(outdated))
	}
}

func TestComputeOutdated_MaxAgeOnlyNoSalvagePossible(t *testing.T) {
	// No MinCount set -> no floor to salvage up to, so every entry past
	// maxAge is deleted regardless of how few remain.
	entries := []Entry{daysAgo(40), daysAgo(41), daysAgo(42)}
	policy := Policy{MaxAge: ptrDuration(30 * 24 * time.Hour)}

	outdated := ComputeOutdated(entries, policy, fixedNow)

	if len(outdated) != 3 {
		t.Fatalf("expected all 3 entries outdated, got %v", names(outdated))
	}
}

func TestComputeOutdated_EmptyInput(t *testing.T) {
	outdated := ComputeOutdated(nil, Policy{MinCount: ptrInt32(3)}, fixedNow)
	if len(outdated) != 0 {
		t.Fatalf("expected no outdated entries for empty input, got %v", names(outdated))
	}
}
