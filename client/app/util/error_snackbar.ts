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
import {CdkPortal, DomPortalOutlet} from '@angular/cdk/portal';
import {HttpErrorResponse} from '@angular/common/http';
import {ApplicationRef, ChangeDetectorRef, Component, ComponentFactoryResolver, Inject, Injector, OnDestroy, OnInit, ViewChild} from '@angular/core';
import {MAT_SNACK_BAR_DATA, MatSnackBar, MatSnackBarRef} from '@angular/material/snack-bar';
import {Subject} from 'rxjs';
import {takeUntil} from 'rxjs/operators';
import {recordHttpErrorMessage, SchedVizError} from './helpers';

/**
 * A window to show the detailed error message in.
 */
@Component({
  selector: 'error-window',
  template: `
    <ng-container *cdkPortal>
      <ng-content></ng-content>
    </ng-container>
  `
})
export class ErrorWindowComponent implements OnInit, OnDestroy {
  @ViewChild(CdkPortal, {static: true}) portal!: CdkPortal;
  private popupWindow: WindowProxy|null = null;

  onClose = new Subject<void>();

  constructor(
      private readonly componentFactoryResolver: ComponentFactoryResolver,
      private readonly applicationRef: ApplicationRef,
      private readonly injector: Injector) {}


  ngOnInit() {
    this.popupWindow = window.open( '');

    const host = new DomPortalOutlet(
        this.popupWindow!.document.body, this.componentFactoryResolver,
        this.applicationRef, this.injector);

    host.attach(this.portal);

    this.popupWindow!.addEventListener('beforeunload', this.close);
  }

  private readonly close = () => {
    this.onClose.next();
  };

  ngOnDestroy() {
    this.close();
    this.onClose.complete();

    if (this.popupWindow) {
      this.popupWindow.close();
    }
  }
}

/**
 * Snack Bar for Error Messages
 */
@Component({
  selector: 'error-snack-bar',
  template: `
    <div>
      <span>{{data.summary}}</span>
      <button mat-button
              color="accent"
              (click)="showMessage()"
              *ngIf="data.error">
        Details
      </button>
      <button mat-button
              color="accent"
              (click)="dismiss()">
        Dismiss
      </button>
    </div>
    <error-window *ngIf="showErrorWindow">
      <head>
        <title>SchedViz Error: {{data.summary}}</title>
      </head>
      <body>
        <pre>{{data.error}}</pre>
      </body>
    </error-window>
  `,
  styles: [`
    span {
      font-family: sans-serif;
    }
  `],
})
export class ErrorSnackBarComponent implements OnDestroy {
  private readonly unsub$ = new Subject<void>();

  @ViewChild(ErrorWindowComponent, {static: false})
  errorWindow!: ErrorWindowComponent;

  showErrorWindow = false;

  constructor(
      private readonly snackBarRef: MatSnackBarRef<ErrorSnackBarComponent>,
      @Inject(MAT_SNACK_BAR_DATA) public data: SchedVizError,
      private readonly changeDetectorRef: ChangeDetectorRef) {}

  dismiss() {
    this.snackBarRef.dismiss();
  }

  showMessage() {
    this.showErrorWindow = true;
    this.changeDetectorRef.detectChanges();
    this.errorWindow.onClose.pipe(takeUntil(this.unsub$)).subscribe(() => {
      this.showErrorWindow = false;
    });
  }

  ngOnDestroy() {
    this.unsub$.next();
    this.unsub$.complete();
  }
}

/**
 * Shows a snackbar for the given error.
 * Snackbar shows message, and has a button to show error
 */
export function showErrorSnackBar(
    snackbar: MatSnackBar, message: string, error: HttpErrorResponse) {
  const err = recordHttpErrorMessage(message, error);
  snackbar.openFromComponent(ErrorSnackBarComponent, {data: err});
}
