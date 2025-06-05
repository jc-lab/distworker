// distworker
// Copyright (C) 2025 JC-Lab
//
// SPDX-License-Identifier: AGPL-3.0-only
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package database

type TaskCollectionStatCounter struct {
	Pending    int64
	Processing int64
	Completed  int64
	Finished   int64
	Error      int64
}

type TaskCollectionStat struct {
	Queues map[string]*TaskCollectionStatCounter
}

func (s *TaskCollectionStat) Total() *TaskCollectionStatCounter {
	total := &TaskCollectionStatCounter{}
	for _, counter := range s.Queues {
		total.Pending += counter.Pending
		total.Processing += counter.Processing
		total.Completed += counter.Completed
		total.Finished += counter.Finished
		total.Error += counter.Error
	}
	return total
}
