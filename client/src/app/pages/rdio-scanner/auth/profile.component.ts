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

import { Component } from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import { MatSnackBar } from '@angular/material/snack-bar';
import { Router } from '@angular/router';
import { AuthService, ProfileAccessCode } from '../../../shared/auth/auth.service';

@Component({
    selector: 'rdio-auth-profile-page',
    styleUrls: ['./profile.component.scss'],
    templateUrl: './profile.component.html',
    standalone: false,
})
export class AuthProfilePageComponent {
    readonly displayedColumns: string[] = ['label', 'code', 'createdAt', 'actions'];

    codes: ProfileAccessCode[] = [];
    loading = false;

    form = new FormBuilder().group({
        label: ['', [Validators.required, Validators.maxLength(120)]],
    });

    constructor(
        private authService: AuthService,
        private matSnackBar: MatSnackBar,
        private router: Router,
    ) {}

    get username(): string {
        return this.authService.username;
    }

    async ngOnInit(): Promise<void> {
        await this.reloadCodes();
    }

    async createCode(): Promise<void> {
        if (this.loading) {
            return;
        }

        if (this.form.invalid) {
            this.form.markAllAsTouched();
            return;
        }

        this.loading = true;

        try {
            const label = this.form.controls.label.value || '';
            const code = await this.authService.createProfileAccessCode(label);
            await this.copyToken(code.code);
            this.matSnackBar.open('Access code generated and copied.', 'Close', { duration: 5000 });
            this.form.reset({ label: '' });
            await this.reloadCodes();

        } catch {
            this.matSnackBar.open('Unable to generate access code.', 'Close', { duration: 5000 });

        } finally {
            this.loading = false;
        }
    }

    async revokeCode(code: ProfileAccessCode): Promise<void> {
        if (this.loading || !code?.id) {
            return;
        }

        this.loading = true;

        try {
            await this.authService.deleteProfileAccessCode(code.id);
            this.matSnackBar.open('Access code revoked.', 'Close', { duration: 5000 });
            await this.reloadCodes();

        } catch {
            this.matSnackBar.open('Unable to revoke access code.', 'Close', { duration: 5000 });

        } finally {
            this.loading = false;
        }
    }

    logout(): void {
        this.authService.logoutLocal();
        this.router.navigateByUrl('/login');
    }

    formatCreatedAt(createdAt: string): string {
        if (!createdAt) {
            return '-';
        }

        const d = new Date(createdAt);
        if (Number.isNaN(d.getTime())) {
            return '-';
        }

        return d.toLocaleString();
    }

    private async reloadCodes(): Promise<void> {
        try {
            this.codes = await this.authService.listProfileAccessCodes();
        } catch {
            this.codes = [];
        }
    }

    private async copyToken(token: string): Promise<void> {
        if (!token) {
            return;
        }

        if (navigator?.clipboard?.writeText) {
            await navigator.clipboard.writeText(token);
            return;
        }

        const textarea = document.createElement('textarea');
        textarea.value = token;
        textarea.style.position = 'fixed';
        textarea.style.opacity = '0';

        document.body.appendChild(textarea);
        textarea.focus();
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
    }
}
