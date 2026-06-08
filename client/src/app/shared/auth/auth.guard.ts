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

import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthService } from './auth.service';
import { PublicConfigService } from './public-config.service';

const toLoginRedirect = (router: Router, returnUrl: string) => {
    return router.createUrlTree(['/login'], {
        queryParams: {
            returnUrl,
        },
    });
};

export const authGuard: CanActivateFn = (_route, state) => {
    const authService = inject(AuthService);
    const router = inject(Router);

    if (authService.isAuthenticated()) {
        return true;
    }

    return toLoginRedirect(router, state.url || '/');
};

export const mainPlayerGuard: CanActivateFn = async (_route, state) => {
    const authService = inject(AuthService);
    const publicConfigService = inject(PublicConfigService);
    const router = inject(Router);

    if (authService.isAuthenticated()) {
        return true;
    }

    if (await publicConfigService.isAnonymousListeningEnabled()) {
        return true;
    }

    return toLoginRedirect(router, state.url || '/');
};

export const adminPortalGuard: CanActivateFn = (_route, state) => {
    const authService = inject(AuthService);
    const router = inject(Router);

    if (!authService.isAuthenticated()) {
        return toLoginRedirect(router, state.url || '/');
    }

    if (authService.canAccessAdminPortal()) {
        return true;
    }

    return router.createUrlTree(['/']);
};

export const strictAdminGuard: CanActivateFn = () => {
    const authService = inject(AuthService);
    const router = inject(Router);

    if (!authService.isAuthenticated()) {
        return toLoginRedirect(router, '/admin');
    }

    if (authService.isAdmin()) {
        return true;
    }

    return router.createUrlTree(['/admin/users']);
};
