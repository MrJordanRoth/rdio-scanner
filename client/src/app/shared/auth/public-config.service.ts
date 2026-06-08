import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { firstValueFrom } from 'rxjs';

type PublicConfigResponse = {
    anonymousListening?: boolean;
};

@Injectable({ providedIn: 'root' })
export class PublicConfigService {
    constructor(private httpClient: HttpClient) {}

    async isAnonymousListeningEnabled(): Promise<boolean> {
        try {
            const config = await firstValueFrom(this.httpClient.get<PublicConfigResponse>('/api/public/config'));
            return config?.anonymousListening === true;
        } catch {
            return false;
        }
    }
}
