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
import {CollectionsFilter} from '../models/collections_filter';

/**
 * Key for the owner property in the URL hash on the collections page
 */
export const OWNER_KEY: 'owner' = 'owner';
/**
 * Key for the creation time property in the URL hash on the collections page
 */
export const CREATION_TIME_KEY: 'creationTime' = 'creationTime';
/**
 * Key for the description property in the URL hash on the collections page
 */
export const DESCRIPTION_KEY: 'description' = 'description';
/**
 * Key for the name property in the URL hash on the collections page
 */
export const NAME_KEY: 'name' = 'name';
/**
 * Key for the tags property in the URL hash on the collections page
 */
export const TAGS_KEY: 'tags' = 'tags';
/**
 * Key for the target machine property in the URL hash on the collections page
 */
export const TARGET_MACHINE_KEY: 'targetMachine' = 'targetMachine';

/**
 * All the keys used for filtering on the collections page.
 */
export const COLLECTIONS_FILTER_KEYS: Array<keyof CollectionsFilter> = [
  CREATION_TIME_KEY, DESCRIPTION_KEY, NAME_KEY, TAGS_KEY, TARGET_MACHINE_KEY
];
/**
 * All the keys used on the collections page.
 */
export const COLLECTIONS_PAGE_KEYS = [OWNER_KEY, ...COLLECTIONS_FILTER_KEYS];

/**
 * Key for the collection name property in the URL hash on the dashboard page
 */
export const COLLECTION_NAME_KEY: 'collection' = 'collection';
/**
 * Key for the share property in the URL hash on the dashboard page
 */
export const SHARE_KEY: 'share' = 'share';


/**
 * All the keys used on the dashboard page.
 */
export const DASHBOARD_PAGE_KEYS = [
  COLLECTION_NAME_KEY, SHARE_KEY,
];
