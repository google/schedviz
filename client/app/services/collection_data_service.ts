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
import {HttpClient, HttpErrorResponse} from '@angular/common/http';
import {Injectable} from '@angular/core';
import {Observable, of} from 'rxjs';
import {catchError, map, take} from 'rxjs/operators';

import {CollectionMetadata, CollectionParameters} from '../models';
import * as services from '../models/collection_data_services';
import {Metadata} from '../models/events';
import {ComplexSystemTopology, SystemTopology} from '../util';

/**
 * A collection service fetches metadata for a particular collection.
 */
export interface CollectionDataService {

  upload(
      file: File, creator: string, tags: string[], description?: string,
      owners?: string[], creationTime?: Date): Observable<string>;

  deleteCollection(collectionName: string): Observable<string>;

  listCollectionMetadata(owner: string): Observable<CollectionMetadata[]>;

  getCollectionMetadata(collectionName: string): Observable<CollectionMetadata>;

  getCollectionParameters(collectionName: string):
      Observable<CollectionParameters>;


  getSystemTopology(metadata: CollectionParameters): Observable<SystemTopology>;

  editCollection(
      collectionName: string, removeTags: string[], addTags: string[],
      description: string, addOwners: string[]): Observable<string>;
}

const STRING_RESPONSE_HEADERS = {
  // Hack to get around broken typedef
  responseType: 'text' as 'json'
};

/**
 * A collection service that fetches data from the SchedViz server.
 */
@Injectable({providedIn: 'root'})
export class HttpCollectionDataService implements CollectionDataService {
  private readonly listCollectionMetadataUrl = '/list_collection_metadata';
  private readonly getCollectionMetadataUrl = '/get_collection_metadata';
  private readonly collectionParametersUrl = '/get_collection_parameters';
  private readonly systemTopologyUrl = '/get_system_topology';
  private readonly uploadUrl = '/upload';
  private readonly deleteCollectionUrl = '/delete_collection';
  private readonly editCollectionUrl = '/edit_collection';

  constructor(private readonly http: HttpClient) {}



  upload(
      file: File, creator: string, tags: string[], description = '',
      owners = [creator], creationTime?: Date): Observable<string> {
    const req:
        services.CreateCollectionRequest = {creator, owners, tags, description};

    if (creationTime) {
      req.creationTime = creationTime.getTime() * 1e6;
    }


    const formData = new FormData();
    formData.append('request', JSON.stringify(req));
    formData.append('file', file);

    return this.http.post<string>(
        this.uploadUrl, formData, STRING_RESPONSE_HEADERS);
  }

  deleteCollection(collectionName: string): Observable<string> {
    const requestUrl = `${this.deleteCollectionUrl}?request=${collectionName}`;
    return this.http.post<string>(
        requestUrl, collectionName, STRING_RESPONSE_HEADERS);
  }

  listCollectionMetadata(owner: string): Observable<CollectionMetadata[]> {
    const requestUrl = `${this.listCollectionMetadataUrl}?request=${owner}`;
    return this.http.post<Metadata[]>(requestUrl, owner)
        .pipe(
            map(metadataList => metadataList.map(
                    metadata => CollectionMetadata.fromJSON(metadata))),
        );
  }

  getCollectionMetadata(collectionName: string):
      Observable<CollectionMetadata> {
    const requestUrl =
        `${this.getCollectionMetadataUrl}?request=${collectionName}`;
    return this.http.post<Metadata>(requestUrl, collectionName)
        .pipe(
            map(metadata => CollectionMetadata.fromJSON(metadata)),
        );
  }

  getCollectionParameters(collectionName: string):
      Observable<CollectionParameters> {
    const requestUrl =
        `${this.collectionParametersUrl}?request=${collectionName}`;
    return this.http
        .post<services.CollectionParametersResponse>(requestUrl, collectionName)
        .pipe(
            map(res => CollectionParameters.fromJSON(res)),
        );
  }


  getSystemTopology(metadata: CollectionParameters):
      Observable<SystemTopology> {
    const requestUrl = `${this.systemTopologyUrl}?request=${metadata.name}`;
    return this.http
        .post<services.SystemTopologyResponse>(requestUrl, metadata.name)
        .pipe(
            map(systemTopology =>
                    new ComplexSystemTopology(systemTopology, metadata.cpus)),
        );
  }

  editCollection(
      collectionName: string, removeTags: string[], addTags: string[],
      description: string, addOwners: string[]): Observable<string> {
    const req: services.EditCollectionRequest = {
      collectionName,
      removeTags,
      addTags,
      description,
      addOwners,
    };
    return this.http.post<string>(
        this.editCollectionUrl, req, STRING_RESPONSE_HEADERS);
  }
}

/**
 * A collection service that returns mock data.
 */
@Injectable({providedIn: 'root'})
export class LocalCollectionDataService implements CollectionDataService {

  upload(
      file: File, creator: string, tags: string[], description?: string,
      owners?: string[], creationTime?: Date): Observable<string> {
    return of('done').pipe(take(1));
  }

  deleteCollection(collectionName: string): Observable<string> {
    return of('done').pipe(take(1));
  }


  getSystemTopology(metadata: CollectionParameters):
      Observable<SystemTopology> {
    return of(new SystemTopology(metadata.cpus)).pipe(take(1));
  }

  listCollectionMetadata(owner: string): Observable<CollectionMetadata[]> {
    return of([
             new CollectionMetadata(
                 '9aa68fcc-9f6b-4585-b35e-20c3318f25e5_5acbc027_someone', owner,
                 [owner], ['fake', 'collection'], 'fake collection', new Date(),
                 ['sched_switch'], 'localhost'),
             new CollectionMetadata(
                 '9aa68fcc-9f6b-4585-b35e-20c3318f25e5_5acbc027_someone', owner,
                 [owner], ['not', 'real'], 'not real collection', new Date(),
                 ['sched_switch'], '127.0.0.1'),
           ])
        .pipe(take(1));
  }

  getCollectionMetadata(collectionName: string):
      Observable<CollectionMetadata> {
    return of(new CollectionMetadata(
                  collectionName, 'me', ['me'], ['fake', 'collection'],
                  'fake collection', new Date(), ['sched_switch'], 'localhost'))
        .pipe(take(1));
  }

  getCollectionParameters(collectionName: string):
      Observable<CollectionParameters> {
    const startTime = 1540768090000;
    const endTime = 1540768139000;
    const cpuCount = 72;
    const cpus = [];
    for (let i = 0; i < cpuCount; i++) {
      cpus.push(i);
    }
    return of(new CollectionParameters(
                  collectionName, cpus, startTime, endTime, []))
        .pipe(take(1));
  }
  editCollection(
      collectionName: string, removeTags: string[], addTags: string[],
      description: string, addOwners: string[]): Observable<string> {
    return of('done').pipe(take(1));
  }
}
