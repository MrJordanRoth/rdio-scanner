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

import { Component, OnDestroy, OnInit } from '@angular/core';
import { AuthService } from '../../../../shared/auth/auth.service';
import { AdminEvent, Config, RdioScannerAdminService } from '../admin.service';

interface Todo {
    level: 'info' | 'warn';
    message: string;
}

@Component({
    selector: 'rdio-scanner-admin-todos',
    styleUrls: ['./todos.component.scss'],
    templateUrl: './todos.component.html',
    standalone: false
})
export class RdioScannerAdminTodosComponent implements OnDestroy, OnInit {
    todos: Todo[] = [];

    private config: Config | undefined;

    private eventSubscription;

    private passwordNeedChange;

    constructor(
        private adminService: RdioScannerAdminService,
        private authService: AuthService,
    ) {
        this.eventSubscription = this.adminService.event.subscribe(async (event: AdminEvent) => {
            if ('config' in event) {
                this.config = event.config;
            }

            if ('passwordNeedChange' in event) {
                this.passwordNeedChange = event.passwordNeedChange || false;
            }

            this.rebuildTodos();
        });

        this.passwordNeedChange = this.adminService.passwordNeedChange;
    }

    ngOnDestroy(): void {
        this.eventSubscription.unsubscribe();
    }

    ngOnInit(): void {
        this.adminService.getConfig().then((config) => {
            this.config = config;

            this.rebuildTodos();
        });
    }

    private rebuildTodos(): void {
        const todos: Todo[] = [];

        if (this.shouldShowDefaultPasswordWarning()) {
            todos.push({
                level: 'warn',
                message: 'You are using a default password. Please navigate to the User Management -> Users tab to update your credentials.'
            });
        }

        if (!this.config?.systems?.length) {
            todos.push({
                level: 'info',
                message: 'No systems defined. You can define one from the systems menu, or let the auto populate feature create it for you if the global option is turned on.',
            });
        }

        if (!this.config?.apikeys?.length && !this.config?.dirwatch?.length) {
            todos.push({
                level: 'info',
                message: 'No apikeys or dirwatch defined. Set at least one to allow ingesting audio files.',
            });
        }

        this.todos = todos;
    }

    private shouldShowDefaultPasswordWarning(): boolean {
        if (!this.passwordNeedChange) {
            return false;
        }

        const username = (this.authService.username || '').trim().toLowerCase();

        // Restrict this warning to the RBAC bootstrap admin account context.
        return this.authService.isAdmin() && username === 'admin';
    }
}
