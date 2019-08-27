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
 * Copies text to the clipboard via a temporary DOM text element.
 *
 * @param text is the text to copy
 * @return a flag indicating whether the copy succeeded
 */
export function copyToClipboard(text: string): boolean {
  const tempElement = document.createElement('textarea');
  document.body.appendChild(tempElement);
  tempElement.value = text;
  tempElement.select();
  let success: boolean;
  try {
    success = document.execCommand('copy');
  } catch {
    success = false;
  }

  document.body.removeChild(tempElement);
  return success;
}
