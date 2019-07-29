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
import {getDurationInNsFromHumanReadableString} from './duration';

describe('Duration', () => {
  describe('getDurationInNsFromHumanReadableString', () => {
    it('invalid input - empty string', () => {
      const input = '';
      const result = getDurationInNsFromHumanReadableString(input);
      expect(result).toBeNaN();
    });

    it('invalid input - no units', () => {
      const input = '50';
      const result = getDurationInNsFromHumanReadableString(input);
      expect(result).toBeNaN();
    });

    it('valid input - no whitespace', () => {
      const input = '100ns';
      const expectedNs = 100;
      const result = getDurationInNsFromHumanReadableString(input);
      expect(result).toBe(expectedNs);
    });

    it('valid input - nanoseconds', () => {
      const inputNanoseconds = 17;
      const validUnits = ['ns', 'nsec', 'nanosecond', 'nanoseconds'];
      validUnits.forEach((unit) => {
        const durationString = `${inputNanoseconds} ${unit}`;
        const actualNs = getDurationInNsFromHumanReadableString(durationString);
        expect(actualNs).toBe(inputNanoseconds);
      });
    });

    it('valid input - milliseconds', () => {
      const milliseconds = 2.3;
      const expectedNs = 1E6 * milliseconds;
      const validUnits = ['ms', 'msec', 'millisecond', 'milliseconds'];
      validUnits.forEach((unit) => {
        const durationString = `${milliseconds} ${unit}`;
        const actualNs = getDurationInNsFromHumanReadableString(durationString);
        expect(actualNs).toBe(expectedNs);
      });
    });

    it('valid input - microseconds', () => {
      const microseconds = 94;
      const expectedNs = 1E3 * microseconds;
      const validUnits =
          ['\u03BCsec', '\u03BCs', 'us', 'usec', 'microsecond', 'microseconds'];
      validUnits.forEach((unit) => {
        const durationString = `${microseconds} ${unit}`;
        const actualNs = getDurationInNsFromHumanReadableString(durationString);
        expect(actualNs).toBe(expectedNs);
      });
    });

    it('valid input - seconds', () => {
      const seconds = 0.21;
      const expectedNs = 1E9 * seconds;
      const validUnits = ['s', 'sec', 'second', 'seconds'];
      validUnits.forEach((unit) => {
        const durationString = `${seconds} ${unit}`;
        const actualNs = getDurationInNsFromHumanReadableString(durationString);
        expect(actualNs).toBe(expectedNs);
      });
    });

    it('valid input - minutes', () => {
      const minutes = 8.017;
      const expectedNs = 60 * 1E9 * minutes;
      const validUnits = ['m', 'min', 'minute', 'minutes'];
      validUnits.forEach((unit) => {
        const durationString = `${minutes} ${unit}`;
        const actualNs = getDurationInNsFromHumanReadableString(durationString);
        expect(actualNs).toBe(expectedNs);
      });
    });

    it('valid input - hours', () => {
      const hours = 6.89;
      const expectedNs = 60 * 60 * 1E9 * hours;
      const validUnits = ['h', 'hr', 'hour', 'hours'];
      validUnits.forEach((unit) => {
        const durationString = `${hours} ${unit}`;
        const actualNs = getDurationInNsFromHumanReadableString(durationString);
        expect(actualNs).toBe(expectedNs);
      });
    });
  });
});
