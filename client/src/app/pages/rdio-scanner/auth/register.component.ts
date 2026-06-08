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

import { Component, inject } from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import { MatSnackBar } from '@angular/material/snack-bar';
import { Router } from '@angular/router';
import { AuthService } from '../../../shared/auth/auth.service';

@Component({
    selector: 'rdio-auth-register-page',
    styleUrls: ['./register.component.scss'],
    templateUrl: './register.component.html',
    standalone: false,
})
export class AuthRegisterPageComponent {
    private readonly formBuilder = inject(FormBuilder);

    form = this.formBuilder.group({
        inviteCode: this.formBuilder.nonNullable.control(''),
        password: this.formBuilder.nonNullable.control('', [Validators.required, Validators.minLength(8)]),
        username: this.formBuilder.nonNullable.control('', [Validators.required]),
    });

    loading = false;

    constructor(
        private authService: AuthService,
        private matSnackBar: MatSnackBar,
        private router: Router,
    ) {}

    async submit(): Promise<void> {
        if (this.form.invalid || this.loading) {
            return;
        }

        this.loading = true;

        try {
            await this.authService.register(
                this.form.controls.username.value,
                this.form.controls.password.value,
                this.form.controls.inviteCode.value,
            );

            this.matSnackBar.open('Registration successful. Please login.', 'Close', { duration: 5000 });
            await this.router.navigateByUrl('/login');

        } catch {
            this.matSnackBar.open('Registration failed. Check your data and invite code.', 'Close', { duration: 5000 });

        } finally {
            this.loading = false;
        }
    }
}
