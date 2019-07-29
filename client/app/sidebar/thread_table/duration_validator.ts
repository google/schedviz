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
import {Directive} from '@angular/core';
import {AbstractControl, NG_VALIDATORS, Validator} from '@angular/forms';

import {getDurationInNsFromHumanReadableString} from '../../util/duration';

/**
 * Validates that a given control contains a valid duration value.
 */
@Directive({
  selector: '[isDuration]',
  providers:
      [{provide: NG_VALIDATORS, useExisting: DurationValidator, multi: true}]
})
export class DurationValidator implements Validator {
  validate(control: AbstractControl): {[key: string]: string} | null {
    if (!control.value || isNaN(
            getDurationInNsFromHumanReadableString(control.value))) {
      return {'invalid': 'invalid input'};
    }

    return null;
  }
}
