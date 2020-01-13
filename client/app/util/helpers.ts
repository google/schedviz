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
/**
 * Most of these functions are copied from Closure
 *
 * Files used:
 * https://github.com/google/closure-library/blob/de66a8ac885a31778404af842ad0f1f9dd595609/closure/goog/crypt/crypt.js
 * https://github.com/google/closure-library/blob/de66a8ac885a31778404af842ad0f1f9dd595609/closure/goog/math/math.js
 * https://github.com/google/closure-library/blob/de66a8ac885a31778404af842ad0f1f9dd595609/closure/goog/crypt/base64.js
 * https://github.com/google/closure-library/blob/de66a8ac885a31778404af842ad0f1f9dd595609/closure/goog/string/string.js
 * https://github.com/google/closure-library/blob/de66a8ac885a31778404af842ad0f1f9dd595609/closure/goog/functions/functions.js
 */

import {HttpErrorResponse} from '@angular/common/http';

/**
 * Tests whether the two values are equal to each other, within a certain
 * tolerance to adjust for floating point errors.
 * @param a A number.
 * @param b A number.
 * @param opt_tolerance Optional tolerance range. Defaults
 *     to 0.000001. If specified, should be greater than 0.
 * @return Whether `a` and `b` are nearly equal.
 */
export function nearlyEquals(
    a: number, b: number, optTolerance: number): boolean {
  return Math.abs(a - b) <= (optTolerance || 0.000001);
}

/**
 * Converts an HttpErrorResponse into a string for suitable for display.
 * @param message A message to prepend to the error
 * @param error An Http Error
 */
export function createHttpErrorMessage(
    message: string, error: HttpErrorResponse) {
  let fullMessage = message;
  if (error.error) {
    fullMessage += `\nReason:\n ${error.error}`;
  } else if (error.message) {
    fullMessage += `\nReason:\n ${error.error}`;
  }
  return fullMessage;
}

/**
 * This function is only here due to the Bazel dev rules generating ES5.
 *
 * @param arr An array
 * @param pred A predicate function to execute on each value in the array
 *             Takes 3 arguments:
 *                current element, current index, and the entire array
 * @return The index of the first element in the array that matches the
 *     predicate
 *          or -1 if no element matched the predicate.
 */
export function findIndex<T>(
    arr: T[],
    pred: (element: T, index?: number, array?: T[]) => boolean): number {
  for (let i = 0; i < arr.length; i++) {
    if (pred(arr[i], i, arr)) {
      return i;
    }
  }
  return -1;
}

/**
 * Turns a string into an array of bytes; a "byte" being a JS number in the
 * range 0-255. Multi-byte characters are written as little-endian.
 * @param str String value to arrify.
 * @return Array of numbers corresponding to the
 *     UCS character codes of each character in str.
 */
export function stringToByteArray(str: string): number[] {
  const output = [];
  let p = 0;
  for (let i = 0; i < str.length; i++) {
    let c = str.charCodeAt(i);
    // NOTE: c <= 0xffff since JavaScript strings are UTF-16.
    if (c > 0xff) {
      output[p++] = c & 0xff;
      c >>= 8;
    }
    output[p++] = c;
  }
  return output;
}

/**
 * Turns an array of numbers into the string given by the concatenation of the
 * characters to which the numbers correspond.
 * @param bytes Array of numbers representing
 *     characters.
 * @return Stringification of the array.
 */
export function byteArrayToString(bytes: Uint8Array|number[]): string {
  const CHUNK_SIZE = 8192;

  // Special-case the simple case for speed's sake.
  if (bytes.length <= CHUNK_SIZE) {
    // TODO(tracked):  Argument of type 'number[] | Uint8Array' is not
    // assignable to parameter of type 'number[]'.
    return String.fromCharCode.apply(null, bytes as any);
  }

  // The remaining logic splits conversion by chunks since
  // Function#apply() has a maximum parameter count.
  // See discussion: http://goo.gl/LrWmZ9

  let str = '';
  for (let i = 0; i < bytes.length; i += CHUNK_SIZE) {
    const chunk = Array.prototype.slice.call(bytes, i, i + CHUNK_SIZE);
    str += String.fromCharCode.apply(null, chunk);
  }
  return str;
}

// Static lookup maps, lazily populated by init_()


/**
 * Maps bytes to characters.
 */
let byteToCharMap_: {[k: string]: string}|null = null;


/**
 * Maps characters to bytes. Used for normal and websafe characters.
 */
let charToByteMap_: {[k: string]: number}|null = null;


/**
 * Maps bytes to websafe characters.
 */
let byteToCharMapWebSafe_: {[k: string]: string}|null = null;


/**
 * Our default alphabet, shared between
 * ENCODED_VALS and ENCODED_VALS_WEBSAFE
 */
const ENCODED_VALS_BASE = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' +
    'abcdefghijklmnopqrstuvwxyz' +
    '0123456789';


/**
 * Our default alphabet. Value 64 (=) is special; it means "nothing."
 */
const ENCODED_VALS = ENCODED_VALS_BASE + '+/=';


/**
 * Our websafe alphabet.
 */
const ENCODED_VALS_WEBSAFE = ENCODED_VALS_BASE + '-_.';

/**
 * Base64-encode an array of bytes.
 *
 * @param input An array of bytes (numbers with
 *     value in [0, 255]) to encode.
 * @param optWebSafe True indicates we should use the alternative
 *     alphabet, which does not require escaping for use in URLs.
 * @return The base64 encoded string.
 */
export function encodeByteArray(
    input: number[]|Uint8Array, optWebSafe = false) {
  base64Init_();

  const byteToCharMap = optWebSafe ? byteToCharMapWebSafe_ : byteToCharMap_;

  const output = [];

  for (let i = 0; i < input.length; i += 3) {
    const byte1 = input[i];
    const haveByte2 = i + 1 < input.length;
    const byte2 = haveByte2 ? input[i + 1] : 0;
    const haveByte3 = i + 2 < input.length;
    const byte3 = haveByte3 ? input[i + 2] : 0;

    const outByte1 = byte1 >> 2;
    const outByte2 = ((byte1 & 0x03) << 4) | (byte2 >> 4);
    let outByte3 = ((byte2 & 0x0F) << 2) | (byte3 >> 6);
    let outByte4 = byte3 & 0x3F;

    if (!haveByte3) {
      outByte4 = 64;

      if (!haveByte2) {
        outByte3 = 64;
      }
    }

    output.push(
        byteToCharMap![outByte1], byteToCharMap![outByte2],
        byteToCharMap![outByte3], byteToCharMap![outByte4]);
  }

  return output.join('');
}

/**
 * Base64-decode a string to an Array of numbers.
 *
 * In base-64 decoding, groups of four characters are converted into three
 * bytes.  If the encoder did not apply padding, the input length may not
 * be a multiple of 4.
 *
 * In this case, the last group will have fewer than 4 characters, and
 * padding will be inferred.  If the group has one or two characters, it decodes
 * to one byte.  If the group has three characters, it decodes to two bytes.
 *
 * @param input Input to decode. Any whitespace is ignored, and the
 *     input maybe encoded with either supported alphabet (or a mix thereof).
 * @return bytes representing the decoded value.
 */
export function decodeStringToByteArray(input: string): number[] {
  const output: number[] = [];
  decodeStringInternal_(input, b => output.push(b));
  return output;
}


/**
 * @paraminput Input to decode.
 * @param pushByte result accumulator.
 */
function decodeStringInternal_(input: string, pushByte: (b: number) => void) {
  base64Init_();

  let nextCharIndex = 0;
  /**
   * @param defaultVal Used for end-of-input.
   * @return The next 6-bit value, or the default for end-of-input.
   */
  function getByte(defaultVal: number): number {
    while (nextCharIndex < input.length) {
      const ch = input.charAt(nextCharIndex++);
      const b = charToByteMap_![ch];
      if (b != null) {
        return b;  // Common case: decoded the char.
      }
      if (!isEmptyOrWhitespace(ch)) {
        throw new Error('Unknown base64 encoding at char: ' + ch);
      }
      // We encountered whitespace: loop around to the next input char.
    }
    return defaultVal;  // No more input remaining.
  }

  while (true) {
    const byte1 = getByte(-1);
    const byte2 = getByte(0);
    const byte3 = getByte(64);
    const byte4 = getByte(64);

    // The common case is that all four bytes are present, so if we have byte4
    // we can skip over the truncated input special case handling.
    if (byte4 === 64) {
      if (byte1 === -1) {
        return;  // Terminal case: no input left to decode.
      }
      // Here we know an intermediate number of bytes are missing.
      // The defaults for byte2, byte3 and byte4 apply the inferred padding
      // rules per the public API documentation. i.e: 1 byte
      // missing should yield 2 bytes of output, but 2 or 3 missing bytes yield
      // a single byte of output. (Recall that 64 corresponds the padding char).
    }

    const outByte1 = (byte1 << 2) | (byte2 >> 4);
    pushByte(outByte1);

    if (byte3 !== 64) {
      const outByte2 = ((byte2 << 4) & 0xF0) | (byte3 >> 2);
      pushByte(outByte2);

      if (byte4 !== 64) {
        const outByte3 = ((byte3 << 6) & 0xC0) | byte4;
        pushByte(outByte3);
      }
    }
  }
}

/**
 * Lazy static initialization function. Called before
 * accessing any of the static map constiables.
 */
function base64Init_() {
  if (!byteToCharMap_) {
    byteToCharMap_ = {};
    charToByteMap_ = {};
    byteToCharMapWebSafe_ = {};

    // We want quick mappings back and forth, so we precompute two maps.
    for (let i = 0; i < ENCODED_VALS.length; i++) {
      byteToCharMap_[i] = ENCODED_VALS.charAt(i);
      charToByteMap_[byteToCharMap_[i]] = i;
      byteToCharMapWebSafe_[i] = ENCODED_VALS_WEBSAFE.charAt(i);

      // Be forgiving when decoding and correctly decode both encodings.
      if (i >= ENCODED_VALS_BASE.length) {
        charToByteMap_[ENCODED_VALS_WEBSAFE.charAt(i)] = i;
      }
    }
  }
}

/**
 * Checks if a string is empty or contains only whitespaces.
 * @param str The string to check.
 * @return Whether `str` is empty or whitespace only.
 */
function isEmptyOrWhitespace(str: string): boolean {
  // testing length == 0 first is actually slower in all browsers (about the
  // same in Opera).
  // Since IE doesn't include non-breaking-space (0xa0) in their \s character
  // class (as required by section 7.2 of the ECMAScript spec), we explicitly
  // include it in the regexp to enforce consistent cross-browser behavior.
  return /^[\s\xa0]*$/.test(str);
}

/**
 * Polyfill for Reflect.has and Reflect.get
 * Only supports basic features; does not support Proxies for example.
 */
// tslint:disable-next-line:variable-name must be call Reflect
export const Reflect = {
  has: (obj: unknown, key: string) => {
    return key in (obj as {[k: string]: string});
  },
  get: (obj: unknown, key: string): unknown => {
    return (obj as {[k: string]: string})[key];
  }
};

/**
 * Wraps a function to allow it to be called, at most, once per interval
 * (specified in milliseconds). If the wrapper function is called N times in
 * that interval, both the 1st and the Nth calls will go through.
 *
 * @param func is the function to throttle
 * @param intervalMs is the interval to throttle the function against
 */
export function throttle<T extends(...args: unknown[]) => void>(
    func: T, intervalMs: number): T {
  let timeout = 0;
  let shouldFire = false;
  let latestArgs: unknown = [];

  const handleTimeout = () => {
    timeout = 0;
    if (shouldFire) {
      shouldFire = false;
      fire();
    }
  };

  const fire = () => {
    timeout = setTimeout(handleTimeout, intervalMs);
    func(latestArgs);
  };

  return ((args) => {
           latestArgs = args;
           if (!timeout) {
             fire();
           } else {
             shouldFire = true;
           }
         }) as T;
}
