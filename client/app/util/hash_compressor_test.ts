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
import {compress, decompress, parseHashFragment, serializeHashFragment} from './hash_compressor';

describe('HashCompressor', () => {
  it('should compress and decompress an object', () => {
    interface DecompressType {
      a: number;
      b: number[];
    }
    const inObj: DecompressType|undefined = {
      a: 1,
      b: [2, 3, 4],
    };
    const compressStr = compress(inObj);
    expect(compressStr).toBe('eJyrVkpUsjLUUUpSsoo20jHWMYmtBQAuZQS%2B');
    const outObj = decompress<DecompressType>(compressStr);
    expect(outObj).toEqual(inObj);
  });

  it('should parse and serialize hashes', () => {
    const hashes: Array<[string, {[k: string]: string}]> = [
      ['', {}],
      ['#abc', {'abc': ''}],
      ['#abc&xyz=456', {'abc': '', 'xyz': '456'}],
      ['#abc=123', {'abc': '123'}],
      ['#abc=123&xyz=456', {'abc': '123', 'xyz': '456'}],
      [
        '#abc=123%204&def=567&xyz=890',
        {'abc': '123 4', 'def': '567', 'xyz': '890'}
      ],
    ];

    // These cases are only for parseHashFragment().
    const parseHashes: Array<[string, {[k: string]: string}]> = [
      ['#abc=123&xyz=456&xyz=789', {'abc': '123', 'xyz': '789'}],
    ];
    for (const hash of parseHashes) {
      expect(parseHashFragment(hash[0])).toEqual(hash[1]);
    }

    for (const hash of hashes) {
      expect(parseHashFragment(hash[0])).toEqual(hash[1]);
      expect(serializeHashFragment(hash[1])).toEqual(hash[0]);
    }
  });
});
