import { Component, OnInit, inject } from '@angular/core';
import { AbstractControl, FormBuilder, ValidationErrors, ValidatorFn, Validators } from '@angular/forms';
import { MatSnackBar } from '@angular/material/snack-bar';
import { AdminRole, AdminRoleScopeSystem, AdminRoleScopeTalkgroup, AdminUser, UserManagementService } from '../user-management.service';

const ROLE_DELAY_MINUTES_MIN = 1;
const ROLE_DELAY_MINUTES_MAX = 10;

@Component({
    selector: 'rdio-scanner-admin-roles-page',
    styleUrls: ['./roles.component.scss'],
    templateUrl: './roles.component.html',
    standalone: false,
})
export class RdioScannerAdminRolesPageComponent implements OnInit {
    private readonly formBuilder = inject(FormBuilder);

    readonly displayedColumns: string[] = ['name', 'description', 'actions'];

    roles: AdminRole[] = [];
    scopeSystems: AdminRoleScopeSystem[] = [];
    scopeTalkgroups: AdminRoleScopeTalkgroup[] = [];
    users: AdminUser[] = [];
    selectedRoleId: number | null = null;

    form = this.formBuilder.group({
        connectionLimit: this.formBuilder.nonNullable.control(0, [Validators.required, this.validateNonNegativeInteger()]),
        delayMinutes: this.formBuilder.nonNullable.control(0, [Validators.required, this.validateDelayMinutes()]),
        description: this.formBuilder.nonNullable.control(''),
        managers: this.formBuilder.nonNullable.control<number[]>([]),
        name: this.formBuilder.nonNullable.control('', [Validators.required]),
        systems: this.formBuilder.nonNullable.control<number[]>([]),
        talkgroups: this.formBuilder.nonNullable.control<number[]>([]),
    });

    constructor(
        private userManagementService: UserManagementService,
        private matSnackBar: MatSnackBar,
    ) {}

    async ngOnInit(): Promise<void> {
        await this.reload();
    }

    edit(role: AdminRole): void {
        this.selectedRoleId = role.id ?? null;
        this.form.setValue({
            connectionLimit: this.normalizeConnectionLimit(role.connectionLimit),
            delayMinutes: this.delaySecondsToMinutes(role.delaySeconds),
            description: role.description || '',
            managers: this.cloneIds(role.managers),
            name: role.name || '',
            systems: this.cloneIds(role.systems),
            talkgroups: this.cloneIds(role.talkgroups),
        });
    }

    resetForm(): void {
        this.selectedRoleId = null;
        this.form.reset({ connectionLimit: 0, delayMinutes: 0, description: '', managers: [], name: '', systems: [], talkgroups: [] });
    }

    async save(): Promise<void> {
        if (this.form.invalid) {
            this.form.markAllAsTouched();
            return;
        }

        const value = this.form.getRawValue();
        const payload = {
            connectionLimit: this.normalizeConnectionLimit(value.connectionLimit),
            delaySeconds: this.delayMinutesToSeconds(value.delayMinutes),
            description: value.description,
            managers: this.cloneIds(value.managers),
            name: value.name,
            systems: this.cloneIds(value.systems),
            talkgroups: this.cloneIds(value.talkgroups),
        };

        try {
            if (this.selectedRoleId) {
                await this.userManagementService.updateRole(this.selectedRoleId, payload);
                this.matSnackBar.open('Role updated.', 'Close', { duration: 4000 });
            } else {
                await this.userManagementService.createRole(payload);
                this.matSnackBar.open('Role created.', 'Close', { duration: 4000 });
            }

            this.resetForm();
            await this.reload();
        } catch {
            this.matSnackBar.open('Unable to save role.', 'Close', { duration: 5000 });
        }
    }

    async delete(role: AdminRole): Promise<void> {
        if (!role.id) {
            return;
        }

        try {
            await this.userManagementService.deleteRole(role.id);
            this.matSnackBar.open('Role deleted.', 'Close', { duration: 4000 });
            await this.reload();
        } catch {
            this.matSnackBar.open('Unable to delete role.', 'Close', { duration: 5000 });
        }
    }

    private async reload(): Promise<void> {
        this.users = await this.userManagementService.listUsers();
        this.roles = await this.userManagementService.listRoles();

        const scopes = await this.userManagementService.listRoleScopes();
        this.scopeSystems = scopes.systems;
        this.scopeTalkgroups = scopes.talkgroups;
    }

    getTalkgroupsForSystem(systemId: number): AdminRoleScopeTalkgroup[] {
        return this.scopeTalkgroups.filter((talkgroup) => talkgroup.systemId === systemId);
    }

    onSystemSelectionChange(systemIds: number[]): void {
        const selected = new Set(systemIds || []);
        const talkgroups = this.cloneIds(this.form.controls.talkgroups.value).filter((talkgroupId) => {
            const talkgroup = this.scopeTalkgroups.find((item) => item.id === talkgroupId);
            return !!talkgroup && selected.has(talkgroup.systemId);
        });

        this.form.controls.talkgroups.setValue(talkgroups);
    }

    private delayMinutesToSeconds(minutes: number): number {
        if (!Number.isFinite(minutes) || minutes <= 0) {
            return 0;
        }

        if (minutes < ROLE_DELAY_MINUTES_MIN || minutes > ROLE_DELAY_MINUTES_MAX) {
            return 0;
        }

        return Math.round(minutes * 60);
    }

    private delaySecondsToMinutes(seconds?: number): number {
        if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds <= 0) {
            return 0;
        }

        if (seconds < ROLE_DELAY_MINUTES_MIN * 60 || seconds > ROLE_DELAY_MINUTES_MAX * 60) {
            return 0;
        }

        return Math.round(seconds / 60);
    }

    private normalizeConnectionLimit(limit?: number): number {
        if (typeof limit !== 'number' || !Number.isFinite(limit) || limit < 0) {
            return 0;
        }

        return Math.floor(limit);
    }

    private cloneIds(ids?: number[]): number[] {
        if (!Array.isArray(ids)) {
            return [];
        }

        return ids
            .filter((id) => typeof id === 'number' && Number.isFinite(id) && id > 0)
            .map((id) => Math.floor(id));
    }

    private validateDelayMinutes(): ValidatorFn {
        return (control: AbstractControl): ValidationErrors | null => {
            const value = control.value;

            if (value === null || value === undefined || value === '') {
                return { required: true };
            }

            if (typeof value !== 'number' || !Number.isFinite(value)) {
                return { invalid: true };
            }

            if (!Number.isInteger(value) || value < 0) {
                return { invalid: true };
            }

            if (value === 0) {
                return null;
            }

            return value >= ROLE_DELAY_MINUTES_MIN && value <= ROLE_DELAY_MINUTES_MAX ? null : { range: true };
        };
    }

    private validateNonNegativeInteger(): ValidatorFn {
        return (control: AbstractControl): ValidationErrors | null => {
            const value = control.value;

            if (value === null || value === undefined || value === '') {
                return { required: true };
            }

            if (typeof value !== 'number' || !Number.isFinite(value)) {
                return { invalid: true };
            }

            if (!Number.isInteger(value) || value < 0) {
                return { invalid: true };
            }

            return null;
        };
    }
}
