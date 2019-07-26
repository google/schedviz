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
import 'pako/dist/pako.min.js';
declare var pako: { inflate: Function, deflate: Function}
const {inflate, deflate} = pako;

import {byteArrayToString, stringToByteArray} from './helpers';
import {decodeStringToByteArray, encodeByteArray} from './helpers';

/**
 * Stringifies the provided object, deflates it, and encodes the deflated string
 * in base64, returning the encoded result.  If JSON stringification fails,
 * returns an empty string.
 * @param obj The object to encode and compress
 * @return The url-encoded, base64-encoded, zlib-compressed JSON string
 *         containing obj.
 */
export function compress(obj: object): string {
  let str = '';
  try {
    str = JSON.stringify(obj);
  } catch (e) {
    return str;
  }
  const data = stringToByteArray(str);
  const compressedStr = deflate(data);
  return encodeURIComponent(encodeByteArray(compressedStr));
}

/**
 * decompress takes in a url-encoded, base64-encoded, zlib-compressed JSON
 * string and returns the uncompressed value.
 *
 * @tparam T: The type that should be decompressed into.
 *            Must be a type representable in JSON.
 *            The decompressed JSON will be blindly cast to this type.
 * @param compressed: The url-encoded, base64-encoded, zlib-compressed JSON
 *                    string to decompress.
 * @return The decompressed object or undefined if an error occurred.
 */
export function decompress<T = never>(compressed: string): T|undefined {
  // Call decodeURIComponent() until stable.
  let decoded = decodeURIComponent(compressed);
  while (decoded !== (decoded = decodeURIComponent(decoded))) {
  }

  const bytes = decodeStringToByteArray(decoded);
  try {
    const stringified = byteArrayToString(inflate(bytes));
    return JSON.parse(stringified) as T;
  } catch {
    // Do nothing
  }
  return;
}

/**
 * parseHashFragment parses a hash fragment into an object.
 *
 * @param hash The location hash string to parse
 * @return A map from hash key to hash value(s).
 */
export function parseHashFragment(hash: string): {[k: string]: string} {
  const ret: {[k: string]: string} = {};
  const hashRegex = /(?:#|&)([^#=&\s]+(?:=[^#=&\s]+)?)/g;
  const matches = hash.match(hashRegex);
  if (matches && matches.length) {
    for (const match of matches) {
      if (!match) {
        continue;
      }
      const [key, value] = match.slice(1).split('=', 2).map(decodeURIComponent);
      ret[key] = value || '';
    }
  }
  return ret;
}

/**
 * serializeHashFragment serializes an object into a hash fragment.
 *
 * @param hashMap A map from hash key to hash value(s).
 * @return A location hash string
 */
export function serializeHashFragment(hashMap: {[k: string]: string}): string {
  // Sort keys to ensure that the hash doesn't change too drastly
  const keys = Object.keys(hashMap).map(encodeURIComponent).sort();
  if (keys.length) {
    const firstValue = encodeURIComponent(hashMap[keys[0]]);
    const firstKeyValue =
        firstValue ? `#${keys[0]}=${firstValue}` : `#${keys[0]}`;
    return keys.slice(1).reduce((acc, key) => {
      const value = encodeURIComponent(hashMap[key]);
      return value ? `${acc}&${key}=${value}` : `${acc}&${key}`;
    }, firstKeyValue);
  } else {
    return '';
  }
}
