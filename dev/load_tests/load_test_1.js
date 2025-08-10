import http from 'k6/http';
import { check } from 'k6';
import { sleep } from 'k6';

const BASE = 'http://localhost:8080';

const endpoints = [
  '/',
  '/health',
  '/favicon.ico',
  // '/package/elastic_package_registry/1.0.0/',
  '/package/zoom/1.2.1/',
  // '/package/aws/3.3.2/',
  '/package/aws/1.16.4/',
  // '/epr/elastic_package_registry/elastic_package_registry-1.0.0.zip',
  '/epr/zoom/zoom-1.2.1.zip',
  // '/epr/aws/aws-3.3.2.zip',
  '/epr/aws/aws-1.16.4.zip',
];

const searches = [
  '',
  '?all=true&prerelease=true',
  '?all=true&prerelease=false',
  '?spec.min=2.0&spec.max=3.0',
  '?spec.min=2.0&spec.max=3.0&prerelease=true',
  '?prerelease=true&capabilities=security',
  '?prerelease=true&categories=security',
  '?package=aws',
  '?type=integration',
  '?package=aws&all=true',
  '?kibana.version=9.0.0',
  // add more combinations as needed
];

const categories = [
  '',
  '?all=true&prerelease=true',
  '?all=true&prerelease=false',
  '?spec.min=2.0&spec.max=3.0',
  '?spec.min=2.0&spec.max=3.0&prerelease=true',
  '?prerelease=true&categories=security',
  '?prerelease=true&capabilities=security',
  '?kibana.version=9.0.0',
  // add more combinations as needed
];

export default function () {
  endpoints.forEach((path) => {
    const res = http.get(`${BASE}${path}`);
    check(res, { 'status 200': r => r.status === 200 });
  });

  // Test /search with varying query parameters
  // const searchParam = searches[Math.floor(Math.random() * searches.length)];
  // const resSearch = http.get(`${BASE}/search${searchParam}`);
  // check(resSearch, { 'status 200': r => r.status === 200 });
  searches.forEach((path) => {
    const res = http.get(`${BASE}${path}`);
    check(res, { 'status 200': r => r.status === 200 });
  });

  // Similarly for /categories
  // const catParam = categories[Math.floor(Math.random() * searches.length)];
  // const resCategories = http.get(`${BASE}/categories${catParam}`);
  // check(resCategories, { 'status 200': r => r.status === 200 });
  categories.forEach((path) => {
    const res = http.get(`${BASE}${path}`);
    check(res, { 'status 200': r => r.status === 200 });
  });

  sleep(1);
}

