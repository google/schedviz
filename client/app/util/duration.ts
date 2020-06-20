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
import {Pipe, PipeTransform} from '@angular/core';


import {Observable} from 'rxjs';
import {debounceTime, filter, map} from 'rxjs/operators';

/**
 * A duration unit object, with two properties:
 *  - 'div': the factor by which to divide a duration in ns to yield this unit.
 *  - 'units': a list of unit strings for this unit.  The first unit will be
 *    used when generating duration strings.
 */
export interface DurationUnit {
  div: number;
  units: string[];
}

/**
 * This array must be sorted in increasing value of durationUnit.div.
 */
const DURATION_UNITS: DurationUnit[] = [
  {div: 1, units: ['nsec', 'ns', 'nanosecond', 'nanoseconds']},
  {
    div: 1E3,
    units: ['\u03BCsec', '\u03BCs', 'usec', 'us', 'microsecond', 'microseconds']
  },
  {div: 1E6, units: ['msec', 'ms', 'millisecond', 'milliseconds']},
  {div: 1E9, units: ['sec', 's', 'second', 'seconds']},
  {div: 60 * 1E9, units: ['min', 'm', 'minute', 'minutes']},
  {div: 60 * 60 * 1E9, units: ['hour', 'h', 'hr', 'hours']},
];

/**
 * A map from unit name to duration object.
 */
const UNIT_TO_DURATION: {[key: string]: DurationUnit;} = {};
for (const dUnit of DURATION_UNITS) {
  for (const unit of dUnit.units) {
    UNIT_TO_DURATION[unit] = dUnit;
  }
}

/**
 * A duration string is a positive or negative integer or real number in base
 * 10, with an optional exponent, followed by a single sequence of characters
 * with no '.' or whitespace, padded with any number (or no) whitespace before
 * the number, after the sequence, or between the two.
 */
const DURATION_REGEX =
    new RegExp(`^\\s*(-?\\d+(?:\\.\\d+)?(?:[eE]\\d+)?)\\s*([^\\.\\s]+)\\s*$`);



/**
 * Returns an Observable that can be used to convert a human-readable time
 * string (or any time unit) to a (debounced) time in nanoseconds.
 */
export function timeInputToNs(timeInput: Observable<string>):
    Observable<number> {
  return timeInput.pipe(
      debounceTime(300), map(getDurationInNsFromHumanReadableString),
      filter(val => !isNaN(val)));
}

/**
 * Returns the provided duration in ns as a human-readable string.  If the
 * duration can't be determined due to the provided duration not being
 * a number or due to a bad or missing DURATION_UNITS, returns an empty
 * string. If durationInNs is 0, the result should be 0 of the first
 * durationUnit.
 */
export function getHumanReadableDurationFromNs(
    durationInNs: number, unit?: string, labelId = 0,
    fractionalDigits = 3): string {
  const absDuration = Math.abs(durationInNs);
  if (isNaN(absDuration) || DURATION_UNITS == null) {
    return '';
  }

  let durationUnit: DurationUnit;
  if (unit === undefined) {
    durationUnit = getMinDurationUnits(durationInNs);
  } else {
    durationUnit = UNIT_TO_DURATION[unit];
  }

  if (!durationUnit) {
    return '';
  }
  const dispDuration = absDuration / durationUnit.div;
  const [, fractional] = dispDuration.toString().split('.');
  const fractionalLen = fractional != null ? fractional.length : 0;
  const dispDurationStr =
      dispDuration.toFixed(Math.min(fractionalLen, fractionalDigits));
  const abbr = durationUnit.units[labelId];
  return `${durationInNs < 0 ? '-' : ''}${dispDurationStr} ${abbr}`;
}

/**
 * Finds the last unit for which the absolute value of durationInNs is larger
 * than the unit's division factor, or the last unit in DURATION_UNITS if that
 * duration is larger than all units' division factors.  If absDuration is
 * smaller than the first unit's division factor, use the first unit.
 * @return the unit of time
 */
export function getMinDurationUnits(durationInNs: number): DurationUnit {
  const absDuration = Math.abs(durationInNs);
  const unit = DURATION_UNITS.find((durationUnit, idx, arr) => {
    return ((idx + 1 === arr.length) || (absDuration < arr[idx + 1].div));
  });
  return unit ? unit : DURATION_UNITS[0];
}

/**
 * As getHumanReadableDurationFromNs, except:
 *   * returns an array of multiple units if applicable.  For example,
 *     1234567890 ns might yield ['1 sec' '234 msec' '567 μs' '890 nsec'].
 *   * clips units below an optional provided threshold.
 *   * 0 of a given unit is suppressed, unless the provided duration is 0,
 *     in which case 0 of the first durationUnit at or above the requested
 *     threshold is returned (e.g., 0 nsec).
 * As with getHumanReadableDurationFromNs, returns an empty array if the
 * provided duration is not a number or if the provided threshold is bad
 * @param durationInNs
 * @param threshold If specified, a threshold unit; only units equal to or
 *     higher than this level will be emitted.  If a string, the threshold is
 *     assumed to be specified as a unit name; if a number, the threshold is
 *     assumed to be specified in ns, and the threshold is set to the lowest
 *     unit equal to or greater than that numeric threshold. If unspecified,
 *     defaults to ns.
 */
export function getFullHumanReadableDurationFromNs(
    durationInNs: number, threshold?: number): string[] {
  // Determine the threshold.
  let thresholdNs = 1;
  if (threshold !== undefined) {
    const thresholdAsNumber = Number(threshold);
    if (isNaN(thresholdAsNumber)) {
      const thresholdDur = UNIT_TO_DURATION[String(threshold)];
      if (thresholdDur === null) {
        console.error(`specified invalid threshold name ${threshold}`);
      } else {
        thresholdNs = thresholdDur.div;
      }
    } else {
      thresholdNs = thresholdAsNumber;
    }
  }
  const thresholdUnit = DURATION_UNITS.find(el => el.div >= thresholdNs);
  if (!thresholdUnit || thresholdUnit.units.length === 0) {
    return [];
  }
  if (!Number(durationInNs)) {
    return ['0 s'];
  }
  // Special case: if the requested duration is actually less than the
  // threshold, then return 0 of the threshold unit.
  if (durationInNs < thresholdUnit.div) {
    return [getHumanReadableDurationFromNs(0, thresholdUnit.units[0])];
  }
  const ret = [];
  for (let idx = DURATION_UNITS.length - 1; idx >= 0; idx--) {
    const dur = DURATION_UNITS[idx];
    if (durationInNs >= thresholdUnit.div && durationInNs >= dur.div) {
      const rem = durationInNs % dur.div;
      const thisDurationInNs = durationInNs - rem;
      if (durationInNs > 0 && dur.units.length > 0) {
        ret.push(
            getHumanReadableDurationFromNs(thisDurationInNs, dur.units[1], 1));
      }
      durationInNs = rem;
    }
  }
  return ret;
}

/**
 * Attempts to parse the duration provided as a human-readable string,
 * returning the parsed duration in nanoseconds, or NaN if parsing failed.
 * @return The parsed duration in nanoseconds.  NaN if parsing
 *     failed.
 */
export function getDurationInNsFromHumanReadableString(durationString: string):
    number {
  const match = durationString.match(DURATION_REGEX);
  if (!match) {
    // Failed to parse.
    return NaN;
  }
  // The first captured field of the duration regex is the number.
  const num = Number(match[1]);
  if (isNaN(num)) {
    // Captured not-a-number as a number :|
    return NaN;
  }
  // The second captured field of the duration regex is the unit.
  const /** string */ unit = match[2].toUpperCase();
  const durationUnit = DURATION_UNITS.find((durationUnit: DurationUnit) => {
    return durationUnit.units.find((el: string) => {
      return (el.toUpperCase() === unit);
    }) != null;
  });
  if (durationUnit) {
    return num * durationUnit.div;
  }
  // Failed to find the supplied string.
  return NaN;
}

/**
 * FormatTimePipe is a wrapper around getHumanReadableDurationFromNs() for use
 * in templates.
 */
@Pipe({name: 'formatTime'})
export class FormatTimePipe implements PipeTransform {
  transform(value: number, unit?: string): string {
    return getHumanReadableDurationFromNs(value, unit);
  }
}
