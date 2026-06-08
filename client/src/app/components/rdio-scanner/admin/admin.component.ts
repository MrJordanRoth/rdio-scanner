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

import { Component, ViewEncapsulation } from '@angular/core';
import { Router } from '@angular/router';
import { AuthService } from '../../../shared/auth/auth.service';
import { Group, Tag } from './admin.service';

@Component({
    encapsulation: ViewEncapsulation.None,
    selector: 'rdio-scanner-admin',
    styleUrls: ['./admin.component.scss'],
    templateUrl: './admin.component.html',
    standalone: false
})
export class RdioScannerAdminComponent {
    groups: Group[] = [];

    tags: Tag[] = [];

    constructor(
        private authService: AuthService,
        private router: Router,
    ) {}

    async logout(): Promise<void> {
        this.authService.logoutLocal();
        await this.router.navigateByUrl('/login');
    }
}
