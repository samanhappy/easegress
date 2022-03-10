/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package validator

import (
	"net/http"

	"fmt"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/signer"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// Kind is the kind of Validator.
	Kind = "Validator"

	resultInvalid = "invalid"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Validator validates HTTP request.",
	Results:     []string{resultInvalid},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func() filters.Filter {
		return &Validator{}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// Validator is filter Validator.
	Validator struct {
		spec *Spec

		headers   *httpheader.Validator
		jwt       *JWTValidator
		signer    *signer.Signer
		oauth2    *OAuth2Validator
		basicAuth *BasicAuthValidator
	}

	// Spec describes the Validator.
	Spec struct {
		filters.BaseSpec `yaml:",inline"`

		Headers   *httpheader.ValidatorSpec `yaml:"headers,omitempty" jsonschema:"omitempty"`
		JWT       *JWTValidatorSpec         `yaml:"jwt,omitempty" jsonschema:"omitempty"`
		Signature *signer.Spec              `yaml:"signature,omitempty" jsonschema:"omitempty"`
		OAuth2    *OAuth2ValidatorSpec      `yaml:"oauth2,omitempty" jsonschema:"omitempty"`
		BasicAuth *BasicAuthValidatorSpec   `yaml:"basicAuth,omitempty" jsonschema:"omitempty"`
	}
)

// Validate verifies that at least one of the validations is defined.
func (spec Spec) Validate() error {
	if spec == (Spec{}) {
		return fmt.Errorf("none of the validations are defined")
	}
	return nil
}

// Name returns the name of the Validator filter instance.
func (v *Validator) Name() string {
	return v.spec.Name()
}

// Kind returns the kind of Validator.
func (v *Validator) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the Validator
func (v *Validator) Spec() filters.Spec {
	return v.spec
}

// Init initializes Validator.
func (v *Validator) Init(spec filters.Spec) {
	v.spec = spec.(*Spec)
	v.reload()
}

// Inherit inherits previous generation of Validator.
func (v *Validator) Inherit(spec filters.Spec, previousGeneration filters.Filter) {
	previousGeneration.Close()
	v.Init(spec)
}

func (v *Validator) reload() {
	if v.spec.Headers != nil {
		v.headers = httpheader.NewValidator(v.spec.Headers)
	}
	if v.spec.JWT != nil {
		v.jwt = NewJWTValidator(v.spec.JWT)
	}
	if v.spec.Signature != nil {
		v.signer = signer.CreateFromSpec(v.spec.Signature)
	}
	if v.spec.OAuth2 != nil {
		v.oauth2 = NewOAuth2Validator(v.spec.OAuth2)
	}
	if v.spec.BasicAuth != nil {
		v.basicAuth = NewBasicAuthValidator(v.spec.BasicAuth, v.spec.Super())
	}
}

// Handle validates HTTPContext.
func (v *Validator) Handle(ctx context.HTTPContext) string {
	result := v.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (v *Validator) handle(ctx context.HTTPContext) string {
	req := ctx.Request()

	prepareErrorResponse := func(status int, tagPrefix string, err error) {
		ctx.Response().SetStatusCode(status)
		ctx.AddTag(stringtool.Cat(tagPrefix, err.Error()))
	}

	if v.headers != nil {
		if err := v.headers.Validate(req.Header()); err != nil {
			prepareErrorResponse(http.StatusBadRequest, "header validator: ", err)
			return resultInvalid
		}
	}
	if v.jwt != nil {
		if err := v.jwt.Validate(req); err != nil {
			prepareErrorResponse(http.StatusUnauthorized, "JWT validator: ", err)
			return resultInvalid
		}
	}
	if v.signer != nil {
		if err := v.signer.Verify(req.Std()); err != nil {
			prepareErrorResponse(http.StatusUnauthorized, "signature validator: ", err)
			return resultInvalid
		}
	}
	if v.oauth2 != nil {
		if err := v.oauth2.Validate(req); err != nil {
			prepareErrorResponse(http.StatusUnauthorized, "oauth2 validator: ", err)
			return resultInvalid
		}
	}
	if v.basicAuth != nil {
		if err := v.basicAuth.Validate(req); err != nil {
			prepareErrorResponse(http.StatusUnauthorized, "http basic validator: ", err)
			return resultInvalid
		}
	}

	return ""
}

// Status returns status.
func (v *Validator) Status() interface{} { return nil }

// Close closes validations.
func (v *Validator) Close() {
	if v.basicAuth != nil {
		v.basicAuth.Close()
	}
}