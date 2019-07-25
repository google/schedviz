//
// Copyright 2019 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS-IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Work-around for angular material issue with ts_devserver and
// ts_web_test_suite. Material requires `module.id` to be valid. This symbol is
// valid in the production bundle but not in ts_devserver and ts_web_test_suite.
// See https://github.com/angular/material2/issues/13883.
var module = {id: ''};
