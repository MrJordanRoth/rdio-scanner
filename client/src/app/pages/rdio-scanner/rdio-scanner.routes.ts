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

import { Routes } from '@angular/router';
import { adminPortalGuard, authGuard, mainPlayerGuard } from '../../shared/auth/auth.guard';
import { AuthLoginPageComponent } from './auth/login.component';
import { AuthProfilePageComponent } from './auth/profile.component';
import { AuthRegisterPageComponent } from './auth/register.component';
import { RdioScannerMainPageComponent } from './rdio-scanner-main.component';
import { RdioScannerPageComponent } from './rdio-scanner.component';

export const routes: Routes = [
    {
        path: '',
        component: RdioScannerPageComponent,
        children: [
            {
                path: 'login',
                component: AuthLoginPageComponent,
            },
            {
                path: 'register',
                component: AuthRegisterPageComponent,
            },
            {
                path: 'profile',
                canActivate: [authGuard],
                component: AuthProfilePageComponent,
            },
            {
                path: '',
                canActivate: [mainPlayerGuard],
                component: RdioScannerMainPageComponent,
            },
            {
                path: 'reset',
                canActivate: [mainPlayerGuard],
                component: RdioScannerMainPageComponent,
            },
            {
                path: 'admin',
                canActivate: [authGuard, adminPortalGuard],
                loadChildren: () => import('./admin/admin.module').then((module) => module.RdioScannerAdminPageModule),
            },
        ],
    },
];
