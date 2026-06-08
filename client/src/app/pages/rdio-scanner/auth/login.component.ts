/*
 * *****************************************************************************
 * Copyright (C) 2019-2026 Chrystian Huot <chrystian.huot@saubeo.solutions>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>
 * ****************************************************************************
 */

import { Component, OnInit, inject } from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { HttpClient } from '@angular/common/http';
import { MatSnackBar } from '@angular/material/snack-bar';
import { firstValueFrom } from 'rxjs';
import { AuthService } from '../../../shared/auth/auth.service';

@Component({
    selector: 'rdio-auth-login-page',
    styleUrls: ['./login.component.scss'],
    templateUrl: './login.component.html',
    standalone: false,
})
export class AuthLoginPageComponent implements OnInit {
    private readonly formBuilder = inject(FormBuilder);

    form = this.formBuilder.group({
        password: this.formBuilder.nonNullable.control('', [Validators.required]),
        username: this.formBuilder.nonNullable.control('', [Validators.required]),
    });

    anonymousListening = false;

    returnUrl = '/admin';

    loading = false;

    constructor(
        private authService: AuthService,
        private httpClient: HttpClient,
        private matSnackBar: MatSnackBar,
        private router: Router,
        private route: ActivatedRoute,
    ) {}

    async ngOnInit(): Promise<void> {
        const returnUrlParam: string | null = this.route.snapshot.queryParamMap.get('returnUrl');
        this.returnUrl = returnUrlParam && returnUrlParam.trim().startsWith('/') ? returnUrlParam : '/admin';

        try {
            const res = await firstValueFrom(this.httpClient.get<{ anonymousListening?: boolean }>('/api/public/config'));
            this.anonymousListening = res?.anonymousListening ?? false;
        } catch {
            this.anonymousListening = false;
        }
    }

    async submit(): Promise<void> {
        if (this.form.invalid || this.loading) {
            return;
        }

        this.loading = true;

        try {
            await this.authService.login(this.form.controls.username.value, this.form.controls.password.value);
            await this.router.navigateByUrl(this.returnUrl);

        } catch {
            this.authService.logoutLocal();
            this.matSnackBar.open('Unable to login. Verify your username and password.', 'Close', { duration: 5000 });

        } finally {
            this.loading = false;
        }
    }

    async guestLogin(): Promise<void> {
        await this.router.navigateByUrl('/');
    }
}
